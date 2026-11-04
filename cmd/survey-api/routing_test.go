// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	surveysvr "github.com/linuxfoundation/lfx-v2-survey-service/gen/http/survey/server"
	surveysvc "github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	goahttp "goa.design/goa/v3/http"
)

// newRoutingTestHandler mounts the generated survey HTTP server the same way
// main.go does, backed by the given survey client mock.
func newRoutingTestHandler(client *mocks.MockSurveyClient) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	surveyService := service.NewSurveyService(client, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, idmapper.NewNoOpMapper(), logger)
	surveyAPI := NewSurveyAPI(surveyService, &mocks.MockAuthenticator{Principal: "test-user"})

	mux := goahttp.NewMuxer()
	surveyServer := surveysvr.New(surveysvc.NewEndpoints(surveyAPI), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	surveysvr.Mount(mux, surveyServer)
	return mux
}

func serveRoutingRequest(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListSurveyTemplatesRoute_ReturnsTemplates(t *testing.T) {
	h := newRoutingTestHandler(&mocks.MockSurveyClient{
		ListSurveyTemplatesFunc: func(ctx context.Context) ([]itx.SurveyTemplate, error) {
			return []itx.SurveyTemplate{
				{ID: "412349483", Title: "Board Member NPS", Nickname: "Board NPS", Href: "https://api.surveymonkey.com/v3/surveys/412349483"},
			}, nil
		},
	})

	rec := serveRoutingRequest(t, h, "/surveys/templates?project_uid=7cad5a8d-19d0-41a4-81a6-043453daf9ee&committee_uid=b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body []map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON array body, got %q: %v", rec.Body.String(), err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 template, got %d", len(body))
	}
	want := map[string]string{
		"id":       "412349483",
		"title":    "Board Member NPS",
		"nickname": "Board NPS",
		"href":     "https://api.surveymonkey.com/v3/surveys/412349483",
	}
	for k, v := range want {
		if body[0][k] != v {
			t.Errorf("expected %s=%q, got %q", k, v, body[0][k])
		}
	}
}

func TestListSurveyTemplatesRoute_InvalidQueryReturnsBadRequest(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "missing project_uid", target: "/surveys/templates"},
		{name: "invalid project_uid", target: "/surveys/templates?project_uid=not-a-uuid"},
		{name: "invalid committee_uid", target: "/surveys/templates?project_uid=7cad5a8d-19d0-41a4-81a6-043453daf9ee&committee_uid=not-a-uuid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// An unset ListSurveyTemplatesFunc panics, so reaching ITX fails the test.
			h := newRoutingTestHandler(&mocks.MockSurveyClient{})

			rec := serveRoutingRequest(t, h, tt.target)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestGetSurveyRoute_StillResolvesSurveyUID(t *testing.T) {
	var gotSurveyID string
	h := newRoutingTestHandler(&mocks.MockSurveyClient{
		GetSurveyFunc: func(ctx context.Context, surveyID string, _ *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
			gotSurveyID = surveyID
			return nil, domain.NewNotFoundError("survey not found")
		},
	})

	rec := serveRoutingRequest(t, h, "/surveys/b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
	if gotSurveyID != "b03cdbaf-53b1-4d47-bc04-dd7e459dd309" {
		t.Errorf("expected get_survey to receive the survey UID, got %q", gotSurveyID)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 from mocked get_survey, got %d: %s", rec.Code, rec.Body.String())
	}
}
