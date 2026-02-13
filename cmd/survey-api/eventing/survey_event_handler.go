// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

const errKey = "error"

// SurveyDBRaw represents raw survey data from v1 DynamoDB/NATS KV bucket
// This is only used for unmarshaling - numeric fields come as strings from DynamoDB
type SurveyDBRaw struct {
	ID                     string                 `json:"id"`
	SurveyMonkeyID         string                 `json:"survey_monkey_id"`
	IsProjectSurvey        bool                   `json:"is_project_survey"`
	StageFilter            string                 `json:"stage_filter"`
	CreatorUsername        string                 `json:"creator_username"`
	CreatorName            string                 `json:"creator_name"`
	CreatorID              string                 `json:"creator_id"`
	CreatedAt              string                 `json:"created_at"`
	LastModifiedAt         string                 `json:"last_modified_at"`
	LastModifiedBy         string                 `json:"last_modified_by"`
	SurveyTitle            string                 `json:"survey_title"`
	SurveySendDate         string                 `json:"survey_send_date"`
	SurveyCutoffDate       string                 `json:"survey_cutoff_date"`
	SurveyReminderRateDays string                 `json:"survey_reminder_rate_days"` // String in DynamoDB
	SendImmediately        bool                   `json:"send_immediately"`
	EmailSubject           string                 `json:"email_subject"`
	EmailBody              string                 `json:"email_body"`
	EmailBodyText          string                 `json:"email_body_text"`
	CommitteeCategory      string                 `json:"committee_category"`
	Committees             []SurveyCommitteeDBRaw `json:"committees"`
	CommitteeVotingEnabled bool                   `json:"committee_voting_enabled"`
	SurveyStatus           string                 `json:"survey_status"`
	NPSValue               string                 `json:"nps_value"`                // String in DynamoDB
	NumPromoters           string                 `json:"num_promoters"`            // String in DynamoDB
	NumPassives            string                 `json:"num_passives"`             // String in DynamoDB
	NumDetractors          string                 `json:"num_detractors"`           // String in DynamoDB
	TotalRecipients        string                 `json:"total_recipients"`         // String in DynamoDB
	TotalSentRecipients    string                 `json:"total_recipients_sent"`    // String in DynamoDB
	TotalResponses         string                 `json:"total_responses"`          // String in DynamoDB
	TotalRecipientsOpened  string                 `json:"total_recipients_opened"`  // String in DynamoDB
	TotalRecipientsClicked string                 `json:"total_recipients_clicked"` // String in DynamoDB
	TotalDeliveryErrors    string                 `json:"total_delivery_errors"`    // String in DynamoDB
	IsNPSSurvey            bool                   `json:"is_nps_survey"`
	CollectorURL           string                 `json:"collector_url"`
}

// SurveyCommitteeDBRaw represents raw committee data from v1 DynamoDB
type SurveyCommitteeDBRaw struct {
	CommitteeID            string `json:"committee_id"` // v1 SFID
	CommitteeName          string `json:"committee_name"`
	ProjectID              string `json:"project_id"` // v1 SFID
	ProjectName            string `json:"project_name"`
	NPSValue               string `json:"nps_value"`                // String in DynamoDB
	NumPromoters           string `json:"num_promoters"`            // String in DynamoDB
	NumPassives            string `json:"num_passives"`             // String in DynamoDB
	NumDetractors          string `json:"num_detractors"`           // String in DynamoDB
	TotalRecipients        string `json:"total_recipients"`         // String in DynamoDB
	TotalSentRecipients    string `json:"total_recipients_sent"`    // String in DynamoDB
	TotalResponses         string `json:"total_responses"`          // String in DynamoDB
	TotalRecipientsOpened  string `json:"total_recipients_opened"`  // String in DynamoDB
	TotalRecipientsClicked string `json:"total_recipients_clicked"` // String in DynamoDB
	TotalDeliveryErrors    string `json:"total_delivery_errors"`    // String in DynamoDB
}

// handleSurveyUpdate processes a survey update from itx-surveys records
// Returns true if the message should be retried (NAK), false if it should be acknowledged (ACK)
func handleSurveyUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("key", key, "handler", "survey")

	funcLogger.DebugContext(ctx, "processing survey update")

	// Convert v1Data map to survey data with proper v2 format
	surveyData, err := convertMapToSurveyData(ctx, v1Data, idMapper, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to survey")
		return false // Permanent error, ACK and skip
	}

	// Extract the survey UID
	if surveyData.UID == "" {
		funcLogger.ErrorContext(ctx, "missing or invalid uid in survey data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("survey_uid", surveyData.UID)

	// Check if survey has at least one valid parent reference (committee or project)
	hasValidParent := false
	for _, committee := range surveyData.Committees {
		if committee.CommitteeUID != "" || committee.ProjectUID != "" {
			hasValidParent = true
			break
		}
	}

	if !hasValidParent {
		funcLogger.InfoContext(ctx, "skipping survey sync - no valid parent references found")
		return false // Permanent issue, ACK and skip
	}

	// Determine action (created vs updated) by checking if mapping exists
	mappingKey := fmt.Sprintf("survey.%s", surveyData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := publisher.PublishSurveyEvent(ctx, string(indexerAction), surveyData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish survey event")
		// Check if this is a transient error that should be retried
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping to track that we've seen this survey
	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store survey mapping")
		// Don't retry on mapping storage failures
	}

	funcLogger.InfoContext(ctx, "successfully sent survey indexer and access messages")
	return false // Success, ACK the message
}

// convertMapToSurveyData converts v1 survey data to v2 format with proper types and UIDs
func convertMapToSurveyData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	logger *slog.Logger,
) (*domain.SurveyData, error) {
	// Convert map to JSON bytes, then to SurveyDBRaw to handle string fields
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var surveyDB SurveyDBRaw
	if err := json.Unmarshal(jsonBytes, &surveyDB); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON into SurveyDBRaw: %w", err)
	}

	// Build v2 survey data struct
	surveyData := &domain.SurveyData{
		UID:                    surveyDB.ID,
		ID:                     surveyDB.ID,
		SurveyMonkeyID:         surveyDB.SurveyMonkeyID,
		IsProjectSurvey:        surveyDB.IsProjectSurvey,
		StageFilter:            surveyDB.StageFilter,
		CreatorUsername:        surveyDB.CreatorUsername,
		CreatorName:            surveyDB.CreatorName,
		CreatorID:              surveyDB.CreatorID,
		CreatedAt:              surveyDB.CreatedAt,
		LastModifiedAt:         surveyDB.LastModifiedAt,
		LastModifiedBy:         surveyDB.LastModifiedBy,
		SurveyTitle:            surveyDB.SurveyTitle,
		SurveySendDate:         surveyDB.SurveySendDate,
		SurveyCutoffDate:       surveyDB.SurveyCutoffDate,
		SendImmediately:        surveyDB.SendImmediately,
		EmailSubject:           surveyDB.EmailSubject,
		EmailBody:              surveyDB.EmailBody,
		EmailBodyText:          surveyDB.EmailBodyText,
		CommitteeCategory:      surveyDB.CommitteeCategory,
		CommitteeVotingEnabled: surveyDB.CommitteeVotingEnabled,
		SurveyStatus:           surveyDB.SurveyStatus,
		IsNPSSurvey:            surveyDB.IsNPSSurvey,
		CollectorURL:           surveyDB.CollectorURL,
	}

	// Convert string integers to actual ints
	if surveyDB.SurveyReminderRateDays != "" {
		if val, err := strconv.Atoi(surveyDB.SurveyReminderRateDays); err == nil {
			surveyData.SurveyReminderRateDays = val
		} else {
			logger.With(errKey, err, "field", "survey_reminder_rate_days", "value", surveyDB.SurveyReminderRateDays).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.NPSValue != "" {
		if val, err := strconv.Atoi(surveyDB.NPSValue); err == nil {
			surveyData.NPSValue = val
		} else {
			logger.With(errKey, err, "field", "nps_value", "value", surveyDB.NPSValue).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.NumPromoters != "" {
		if val, err := strconv.Atoi(surveyDB.NumPromoters); err == nil {
			surveyData.NumPromoters = val
		} else {
			logger.With(errKey, err, "field", "num_promoters", "value", surveyDB.NumPromoters).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.NumPassives != "" {
		if val, err := strconv.Atoi(surveyDB.NumPassives); err == nil {
			surveyData.NumPassives = val
		} else {
			logger.With(errKey, err, "field", "num_passives", "value", surveyDB.NumPassives).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.NumDetractors != "" {
		if val, err := strconv.Atoi(surveyDB.NumDetractors); err == nil {
			surveyData.NumDetractors = val
		} else {
			logger.With(errKey, err, "field", "num_detractors", "value", surveyDB.NumDetractors).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.TotalRecipients != "" {
		if val, err := strconv.Atoi(surveyDB.TotalRecipients); err == nil {
			surveyData.TotalRecipients = val
		} else {
			logger.With(errKey, err, "field", "total_recipients", "value", surveyDB.TotalRecipients).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.TotalSentRecipients != "" {
		if val, err := strconv.Atoi(surveyDB.TotalSentRecipients); err == nil {
			surveyData.TotalSentRecipients = val
		} else {
			logger.With(errKey, err, "field", "total_sent_recipients", "value", surveyDB.TotalSentRecipients).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.TotalResponses != "" {
		if val, err := strconv.Atoi(surveyDB.TotalResponses); err == nil {
			surveyData.TotalResponses = val
		} else {
			logger.With(errKey, err, "field", "total_responses", "value", surveyDB.TotalResponses).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.TotalRecipientsOpened != "" {
		if val, err := strconv.Atoi(surveyDB.TotalRecipientsOpened); err == nil {
			surveyData.TotalRecipientsOpened = val
		} else {
			logger.With(errKey, err, "field", "total_recipients_opened", "value", surveyDB.TotalRecipientsOpened).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.TotalRecipientsClicked != "" {
		if val, err := strconv.Atoi(surveyDB.TotalRecipientsClicked); err == nil {
			surveyData.TotalRecipientsClicked = val
		} else {
			logger.With(errKey, err, "field", "total_recipients_clicked", "value", surveyDB.TotalRecipientsClicked).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	if surveyDB.TotalDeliveryErrors != "" {
		if val, err := strconv.Atoi(surveyDB.TotalDeliveryErrors); err == nil {
			surveyData.TotalDeliveryErrors = val
		} else {
			logger.With(errKey, err, "field", "total_delivery_errors", "value", surveyDB.TotalDeliveryErrors).
				WarnContext(ctx, "failed to convert string to int")
		}
	}

	// Process committees array
	for _, committeeDB := range surveyDB.Committees {
		committeeData := domain.SurveyCommitteeData{
			CommitteeID:   committeeDB.CommitteeID,
			CommitteeName: committeeDB.CommitteeName,
			ProjectID:     committeeDB.ProjectID,
			ProjectName:   committeeDB.ProjectName,
		}

		// Convert committee string integers
		if committeeDB.NPSValue != "" {
			if val, err := strconv.Atoi(committeeDB.NPSValue); err == nil {
				committeeData.NPSValue = val
			}
		}
		if committeeDB.NumPromoters != "" {
			if val, err := strconv.Atoi(committeeDB.NumPromoters); err == nil {
				committeeData.NumPromoters = val
			}
		}
		if committeeDB.NumPassives != "" {
			if val, err := strconv.Atoi(committeeDB.NumPassives); err == nil {
				committeeData.NumPassives = val
			}
		}
		if committeeDB.NumDetractors != "" {
			if val, err := strconv.Atoi(committeeDB.NumDetractors); err == nil {
				committeeData.NumDetractors = val
			}
		}
		if committeeDB.TotalRecipients != "" {
			if val, err := strconv.Atoi(committeeDB.TotalRecipients); err == nil {
				committeeData.TotalRecipients = val
			}
		}
		if committeeDB.TotalSentRecipients != "" {
			if val, err := strconv.Atoi(committeeDB.TotalSentRecipients); err == nil {
				committeeData.TotalSentRecipients = val
			}
		}
		if committeeDB.TotalResponses != "" {
			if val, err := strconv.Atoi(committeeDB.TotalResponses); err == nil {
				committeeData.TotalResponses = val
			}
		}
		if committeeDB.TotalRecipientsOpened != "" {
			if val, err := strconv.Atoi(committeeDB.TotalRecipientsOpened); err == nil {
				committeeData.TotalRecipientsOpened = val
			}
		}
		if committeeDB.TotalRecipientsClicked != "" {
			if val, err := strconv.Atoi(committeeDB.TotalRecipientsClicked); err == nil {
				committeeData.TotalRecipientsClicked = val
			}
		}
		if committeeDB.TotalDeliveryErrors != "" {
			if val, err := strconv.Atoi(committeeDB.TotalDeliveryErrors); err == nil {
				committeeData.TotalDeliveryErrors = val
			}
		}

		// Map v1 committee ID (SFID) to v2 committee UID
		if committeeDB.CommitteeID != "" {
			committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, committeeDB.CommitteeID)
			if err != nil {
				logger.With(errKey, err, "field", "committee_id", "value", committeeDB.CommitteeID).
					WarnContext(ctx, "failed to get v2 committee UID from v1 committee ID")
				// Don't set committee_uid if mapping fails
			} else {
				committeeData.CommitteeUID = committeeUID
			}
		}

		// Map v1 project ID (SFID) to v2 project UID
		if committeeDB.ProjectID != "" {
			projectUID, err := idMapper.MapProjectV1ToV2(ctx, committeeDB.ProjectID)
			if err != nil {
				logger.With(errKey, err, "field", "project_id", "value", committeeDB.ProjectID).
					WarnContext(ctx, "failed to get v2 project UID from v1 project ID")
				// Don't set project_uid if mapping fails
			} else {
				committeeData.ProjectUID = projectUID
				logger.With("v1_project_id", committeeDB.ProjectID, "v2_project_uid", projectUID).
					DebugContext(ctx, "mapped project v1 ID to v2 UID")
			}
		}

		surveyData.Committees = append(surveyData.Committees, committeeData)
	}

	return surveyData, nil
}

// handleSurveyDelete processes a survey delete from itx-surveys records
// Returns true if the message should be retried (NAK), false if it should be acknowledged (ACK)
func handleSurveyDelete(
	ctx context.Context,
	uid string,
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("survey_uid", uid, "handler", "survey_delete")

	funcLogger.DebugContext(ctx, "processing survey delete")

	// Create minimal survey data for delete event
	surveyData := &domain.SurveyData{
		UID: uid,
		ID:  uid,
	}

	// Publish delete event to indexer and FGA-sync
	if err := publisher.PublishSurveyEvent(ctx, string(indexerConstants.ActionDeleted), surveyData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish survey delete event")
		// Check if this is a transient error that should be retried
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping from v1-mappings KV
	mappingKey := fmt.Sprintf("survey.%s", uid)
	if err := mappingsKV.Delete(ctx, mappingKey); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to delete survey mapping")
		// Don't retry on mapping deletion failures
	}

	funcLogger.InfoContext(ctx, "successfully sent survey delete indexer and access messages")
	return false // Success, ACK the message
}

// isTransientError determines if an error is transient and should be retried
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// NATS publish errors, timeouts, connection issues
	if utils.Contains(errStr, "timeout") || utils.Contains(errStr, "connection") ||
		utils.Contains(errStr, "unavailable") || utils.Contains(errStr, "deadline") {
		return true
	}

	return false
}
