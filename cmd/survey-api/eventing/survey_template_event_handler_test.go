// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templateV1Data is a v1 surveymonkey-surveys record with counters as strings.
func templateV1Data() map[string]any {
	return map[string]any{
		"id":               "412349483",
		"title":            "Board Member NPS",
		"href":             "https://api.surveymonkey.com/v3/surveys/412349483",
		"nickname":         "Board NPS",
		"question_count":   "12",
		"analyze_url":      "https://example.com/analyze",
		"edit_url":         "https://example.com/edit",
		"collect_url":      "https://example.com/collect",
		"preview":          "https://example.com/preview",
		"date_created":     "2030-01-01T00:00:00",
		"date_modified":    "2030-01-02T00:00:00",
		"language":         "en",
		"folder_id":        "folder-1",
		"page_count":       "3",
		"category":         "nps",
		"is_owner":         true,
		"custom_variables": map[string]any{"region": "emea"},
	}
}

func wantTemplateData() *domain.SurveyTemplateData {
	return &domain.SurveyTemplateData{
		ID: "412349483", Title: "Board Member NPS", Href: "https://api.surveymonkey.com/v3/surveys/412349483",
		Nickname: "Board NPS", QuestionCount: 12, AnalyzeUrl: "https://example.com/analyze",
		EditUrl: "https://example.com/edit", CollectUrl: "https://example.com/collect",
		Preview: "https://example.com/preview", DateCreated: "2030-01-01T00:00:00",
		DateModified: "2030-01-02T00:00:00", Language: "en", FolderID: "folder-1", PageCount: 3,
		Category: "nps", IsOwner: true, CustomVariables: map[string]string{"region": "emea"},
	}
}

// ─── SurveyTemplateDBRaw.UnmarshalJSON ───────────────────────────────────────

func TestSurveyTemplateDBRaw_UnmarshalJSON(t *testing.T) {
	t.Run("maps every field and converts numeric strings", func(t *testing.T) {
		raw, err := json.Marshal(templateV1Data())
		require.NoError(t, err)

		var got SurveyTemplateDBRaw
		require.NoError(t, json.Unmarshal(raw, &got))

		want := wantTemplateData()
		assert.Equal(t, SurveyTemplateDBRaw{
			ID: want.ID, Title: want.Title, Href: want.Href, Nickname: want.Nickname,
			QuestionCount: 12, AnalyzeUrl: want.AnalyzeUrl, EditUrl: want.EditUrl, CollectUrl: want.CollectUrl,
			Preview: want.Preview, DateCreated: want.DateCreated, DateModified: want.DateModified,
			Language: want.Language, FolderID: want.FolderID, PageCount: 3, Category: want.Category,
			IsOwner: true, CustomVariables: want.CustomVariables,
		}, got)
	})

	t.Run("accepts JSON numbers, empty strings and null", func(t *testing.T) {
		var got SurveyTemplateDBRaw
		require.NoError(t, json.Unmarshal([]byte(`{"question_count":7,"page_count":""}`), &got))
		assert.Equal(t, 7, got.QuestionCount)
		assert.Equal(t, 0, got.PageCount)

		got = SurveyTemplateDBRaw{}
		require.NoError(t, json.Unmarshal([]byte(`{"question_count":null}`), &got))
		assert.Zero(t, got.QuestionCount)
	})

	for _, field := range []string{"question_count", "page_count"} {
		t.Run("rejects non-numeric "+field, func(t *testing.T) {
			var got SurveyTemplateDBRaw
			err := json.Unmarshal([]byte(`{"`+field+`":"abc"}`), &got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert "+field)
		})
	}

	t.Run("rejects an unsupported counter type", func(t *testing.T) {
		var got SurveyTemplateDBRaw
		err := json.Unmarshal([]byte(`{"page_count":true}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type for numeric field")
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		var got SurveyTemplateDBRaw
		require.Error(t, got.UnmarshalJSON([]byte(`{`)))
	})
}

// ─── convertMapToSurveyTemplateData ──────────────────────────────────────────

func TestConvertMapToSurveyTemplateData(t *testing.T) {
	t.Run("maps every field", func(t *testing.T) {
		got, err := convertMapToSurveyTemplateData(templateV1Data())
		require.NoError(t, err)
		assert.Equal(t, wantTemplateData(), got)
	})

	t.Run("an invalid counter is an error", func(t *testing.T) {
		data := templateV1Data()
		data["question_count"] = "lots"

		_, err := convertMapToSurveyTemplateData(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON into SurveyTemplateDBRaw")
	})

	t.Run("an unmarshalable value is an error", func(t *testing.T) {
		_, err := convertMapToSurveyTemplateData(map[string]any{"id": func() {}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal v1Data")
	})
}

// ─── handleSurveyTemplateUpdate ──────────────────────────────────────────────

func TestHandleSurveyTemplateUpdate(t *testing.T) {
	ctx := context.Background()
	const key = "surveymonkey-surveys.412349483"

	t.Run("publishes created when no mapping exists, then stores the mapping", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)

		assert.False(t, handleSurveyTemplateUpdate(ctx, key, templateV1Data(), pub, kv, discardLogger()))

		require.Len(t, *events, 1)
		assert.Equal(t, "created", (*events)[0].Action)
		assert.Equal(t, wantTemplateData(), (*events)[0].Template)
		assert.Equal(t, "1", kv.value("survey_template.412349483"))
	})

	t.Run("publishes updated when a mapping already exists", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey_template.412349483": "1"})

		handleSurveyTemplateUpdate(ctx, key, templateV1Data(), pub, kv, discardLogger())

		require.Len(t, *events, 1)
		assert.Equal(t, "updated", (*events)[0].Action)
	})

	t.Run("a conversion failure is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := templateV1Data()
		data["page_count"] = "bad"

		assert.False(t, handleSurveyTemplateUpdate(ctx, key, data, pub, newFakeKV(nil), discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a missing id is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := templateV1Data()
		delete(data, "id")

		assert.False(t, handleSurveyTemplateUpdate(ctx, key, data, pub, newFakeKV(nil), discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a transient publish failure is retried and stores no mapping", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrConnectionClosed)
		kv := newFakeKV(nil)

		assert.True(t, handleSurveyTemplateUpdate(ctx, key, templateV1Data(), pub, kv, discardLogger()))
		assert.Empty(t, kv.data)
	})

	t.Run("a permanent publish failure is acknowledged and stores no mapping", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		kv := newFakeKV(nil)

		assert.False(t, handleSurveyTemplateUpdate(ctx, key, templateV1Data(), pub, kv, discardLogger()))
		assert.Empty(t, kv.data)
	})

	t.Run("a mapping store failure after publish is still acknowledged", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)
		kv.putErr = errors.New("kv down")

		assert.False(t, handleSurveyTemplateUpdate(ctx, key, templateV1Data(), pub, kv, discardLogger()))
		assert.Len(t, *events, 1)
	})
}

// ─── handleSurveyTemplateDelete ──────────────────────────────────────────────

func TestHandleSurveyTemplateDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("publishes deleted with the id and tombstones the mapping", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey_template.t1": "1"})

		assert.False(t, handleSurveyTemplateDelete(ctx, "t1", pub, kv, discardLogger()))

		require.Len(t, *events, 1)
		assert.Equal(t, "deleted", (*events)[0].Action)
		assert.Equal(t, &domain.SurveyTemplateData{ID: "t1"}, (*events)[0].Template)
		assert.Equal(t, tombstoneMarker, kv.value("survey_template.t1"))
	})

	t.Run("an already tombstoned template is not published again", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey_template.t1": tombstoneMarker})

		assert.False(t, handleSurveyTemplateDelete(ctx, "t1", pub, kv, discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a live mapping does not suppress the delete", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey_template.t1": "1"})

		handleSurveyTemplateDelete(ctx, "t1", pub, kv, discardLogger())
		assert.Len(t, *events, 1)
	})

	t.Run("a transient publish failure is retried and not tombstoned", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrTimeout)
		kv := newFakeKV(map[string]string{"survey_template.t1": "1"})

		assert.True(t, handleSurveyTemplateDelete(ctx, "t1", pub, kv, discardLogger()))
		assert.Equal(t, "1", kv.value("survey_template.t1"))
	})

	t.Run("a permanent publish failure is acknowledged and not tombstoned", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		kv := newFakeKV(map[string]string{"survey_template.t1": "1"})

		assert.False(t, handleSurveyTemplateDelete(ctx, "t1", pub, kv, discardLogger()))
		assert.Equal(t, "1", kv.value("survey_template.t1"))
	})

	t.Run("a tombstone store failure is still acknowledged", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)
		kv.putErr = errors.New("kv down")

		assert.False(t, handleSurveyTemplateDelete(ctx, "t1", pub, kv, discardLogger()))
		assert.Len(t, *events, 1)
	})
}
