// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
)

// SurveyTemplateDBRaw represents raw survey template data from the surveymonkey-surveys DynamoDB table
type SurveyTemplateDBRaw struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Href            string            `json:"href"`
	Nickname        string            `json:"nickname"`
	QuestionCount   int               `json:"question_count"`
	AnalyzeUrl      string            `json:"analyze_url"`
	EditUrl         string            `json:"edit_url"`
	CollectUrl      string            `json:"collect_url"`
	Preview         string            `json:"preview"`
	DateCreated     string            `json:"date_created"`
	DateModified    string            `json:"date_modified"`
	Language        string            `json:"language"`
	FolderID        string            `json:"folder_id"`
	PageCount       int               `json:"page_count"`
	Category        string            `json:"category"`
	IsOwner         bool              `json:"is_owner"`
	CustomVariables map[string]string `json:"custom_variables"`
}

// handleSurveyTemplateUpdate processes a survey template update from surveymonkey-surveys records
// Returns true if the message should be retried (NAK), false if it should be acknowledged (ACK)
func handleSurveyTemplateUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("key", key, "handler", "survey_template")

	funcLogger.DebugContext(ctx, "processing survey template update")

	templateData, err := convertMapToSurveyTemplateData(v1Data)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to survey template")
		return false // Permanent error, ACK and skip
	}

	if templateData.ID == "" {
		funcLogger.ErrorContext(ctx, "missing or invalid id in survey template data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("template_id", templateData.ID)

	// Determine action (created vs updated) by checking if mapping exists
	mappingKey := fmt.Sprintf("survey_template.%s", templateData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	if err := publisher.PublishSurveyTemplateEvent(ctx, string(indexerAction), templateData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish survey template event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store survey template mapping")
	}

	funcLogger.InfoContext(ctx, "successfully sent survey template indexer message")
	return false // Success, ACK the message
}

// convertMapToSurveyTemplateData converts a raw v1Data map to a SurveyTemplateData struct
func convertMapToSurveyTemplateData(v1Data map[string]interface{}) (*domain.SurveyTemplateData, error) {
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var raw SurveyTemplateDBRaw
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON into SurveyTemplateDBRaw: %w", err)
	}

	return &domain.SurveyTemplateData{
		ID:              raw.ID,
		Title:           raw.Title,
		Href:            raw.Href,
		Nickname:        raw.Nickname,
		QuestionCount:   raw.QuestionCount,
		AnalyzeUrl:      raw.AnalyzeUrl,
		EditUrl:         raw.EditUrl,
		CollectUrl:      raw.CollectUrl,
		Preview:         raw.Preview,
		DateCreated:     raw.DateCreated,
		DateModified:    raw.DateModified,
		Language:        raw.Language,
		FolderID:        raw.FolderID,
		PageCount:       raw.PageCount,
		Category:        raw.Category,
		IsOwner:         raw.IsOwner,
		CustomVariables: raw.CustomVariables,
	}, nil
}

// handleSurveyTemplateDelete processes a survey template delete from surveymonkey-surveys records
// Returns true if the message should be retried (NAK), false if it should be acknowledged (ACK)
func handleSurveyTemplateDelete(
	ctx context.Context,
	uid string,
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("template_id", uid, "handler", "survey_template_delete")

	funcLogger.DebugContext(ctx, "processing survey template delete")

	mappingKey := fmt.Sprintf("survey_template.%s", uid)
	if entry, err := mappingsKV.Get(ctx, mappingKey); err == nil && isTombstonedMapping(entry.Value()) {
		funcLogger.DebugContext(ctx, "survey template delete already processed, skipping")
		return false
	}

	templateData := &domain.SurveyTemplateData{ID: uid}

	if err := publisher.PublishSurveyTemplateEvent(ctx, string(indexerConstants.ActionDeleted), templateData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish survey template delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	if _, err := mappingsKV.Put(ctx, mappingKey, []byte(tombstoneMarker)); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to tombstone survey template mapping")
	}

	funcLogger.InfoContext(ctx, "successfully sent survey template delete indexer message")
	return false // Success, ACK the message
}
