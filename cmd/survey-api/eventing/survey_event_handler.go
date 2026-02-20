// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
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
	SurveyReminderRateDays int                    `json:"survey_reminder_rate_days"`
	SendImmediately        bool                   `json:"send_immediately"`
	EmailSubject           string                 `json:"email_subject"`
	EmailBody              string                 `json:"email_body"`
	EmailBodyText          string                 `json:"email_body_text"`
	CommitteeCategory      string                 `json:"committee_category"`
	Committees             []SurveyCommitteeDBRaw `json:"committees"`
	CommitteeVotingEnabled bool                   `json:"committee_voting_enabled"`
	SurveyStatus           string                 `json:"survey_status"`
	NPSValue               int                    `json:"nps_value"`
	NumPromoters           int                    `json:"num_promoters"`
	NumPassives            int                    `json:"num_passives"`
	NumDetractors          int                    `json:"num_detractors"`
	TotalRecipients        int                    `json:"total_recipients"`
	TotalSentRecipients    int                    `json:"total_recipients_sent"`
	TotalResponses         int                    `json:"total_responses"`
	TotalRecipientsOpened  int                    `json:"total_recipients_opened"`
	TotalRecipientsClicked int                    `json:"total_recipients_clicked"`
	TotalDeliveryErrors    int                    `json:"total_delivery_errors"`
	IsNPSSurvey            bool                   `json:"is_nps_survey"`
	CollectorURL           string                 `json:"collector_url"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
func (s *SurveyDBRaw) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with interface{} types for numeric fields
	tmp := struct {
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
		SurveyReminderRateDays interface{}            `json:"survey_reminder_rate_days"`
		SendImmediately        bool                   `json:"send_immediately"`
		EmailSubject           string                 `json:"email_subject"`
		EmailBody              string                 `json:"email_body"`
		EmailBodyText          string                 `json:"email_body_text"`
		CommitteeCategory      string                 `json:"committee_category"`
		Committees             []SurveyCommitteeDBRaw `json:"committees"`
		CommitteeVotingEnabled bool                   `json:"committee_voting_enabled"`
		SurveyStatus           string                 `json:"survey_status"`
		NPSValue               interface{}            `json:"nps_value"`
		NumPromoters           interface{}            `json:"num_promoters"`
		NumPassives            interface{}            `json:"num_passives"`
		NumDetractors          interface{}            `json:"num_detractors"`
		TotalRecipients        interface{}            `json:"total_recipients"`
		TotalSentRecipients    interface{}            `json:"total_recipients_sent"`
		TotalResponses         interface{}            `json:"total_responses"`
		TotalRecipientsOpened  interface{}            `json:"total_recipients_opened"`
		TotalRecipientsClicked interface{}            `json:"total_recipients_clicked"`
		TotalDeliveryErrors    interface{}            `json:"total_delivery_errors"`
		IsNPSSurvey            bool                   `json:"is_nps_survey"`
		CollectorURL           string                 `json:"collector_url"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Helper function to convert interface{} to int
	convertToInt := func(v interface{}) (int, error) {
		if v == nil {
			return 0, nil
		}
		switch val := v.(type) {
		case string:
			if val == "" {
				return 0, nil
			}
			return strconv.Atoi(val)
		case float64:
			return int(val), nil
		case int:
			return val, nil
		default:
			return 0, fmt.Errorf("invalid type for numeric field: %T", v)
		}
	}

	// Assign string and bool fields directly
	s.ID = tmp.ID
	s.SurveyMonkeyID = tmp.SurveyMonkeyID
	s.IsProjectSurvey = tmp.IsProjectSurvey
	s.StageFilter = tmp.StageFilter
	s.CreatorUsername = tmp.CreatorUsername
	s.CreatorName = tmp.CreatorName
	s.CreatorID = tmp.CreatorID
	s.CreatedAt = tmp.CreatedAt
	s.LastModifiedAt = tmp.LastModifiedAt
	s.LastModifiedBy = tmp.LastModifiedBy
	s.SurveyTitle = tmp.SurveyTitle
	s.SurveySendDate = tmp.SurveySendDate
	s.SurveyCutoffDate = tmp.SurveyCutoffDate
	s.SendImmediately = tmp.SendImmediately
	s.EmailSubject = tmp.EmailSubject
	s.EmailBody = tmp.EmailBody
	s.EmailBodyText = tmp.EmailBodyText
	s.CommitteeCategory = tmp.CommitteeCategory
	s.Committees = tmp.Committees
	s.CommitteeVotingEnabled = tmp.CommitteeVotingEnabled
	s.SurveyStatus = tmp.SurveyStatus
	s.IsNPSSurvey = tmp.IsNPSSurvey
	s.CollectorURL = tmp.CollectorURL

	// Convert numeric fields
	var err error
	if s.SurveyReminderRateDays, err = convertToInt(tmp.SurveyReminderRateDays); err != nil {
		return fmt.Errorf("failed to convert survey_reminder_rate_days: %w", err)
	}
	if s.NPSValue, err = convertToInt(tmp.NPSValue); err != nil {
		return fmt.Errorf("failed to convert nps_value: %w", err)
	}
	if s.NumPromoters, err = convertToInt(tmp.NumPromoters); err != nil {
		return fmt.Errorf("failed to convert num_promoters: %w", err)
	}
	if s.NumPassives, err = convertToInt(tmp.NumPassives); err != nil {
		return fmt.Errorf("failed to convert num_passives: %w", err)
	}
	if s.NumDetractors, err = convertToInt(tmp.NumDetractors); err != nil {
		return fmt.Errorf("failed to convert num_detractors: %w", err)
	}
	if s.TotalRecipients, err = convertToInt(tmp.TotalRecipients); err != nil {
		return fmt.Errorf("failed to convert total_recipients: %w", err)
	}
	if s.TotalSentRecipients, err = convertToInt(tmp.TotalSentRecipients); err != nil {
		return fmt.Errorf("failed to convert total_recipients_sent: %w", err)
	}
	if s.TotalResponses, err = convertToInt(tmp.TotalResponses); err != nil {
		return fmt.Errorf("failed to convert total_responses: %w", err)
	}
	if s.TotalRecipientsOpened, err = convertToInt(tmp.TotalRecipientsOpened); err != nil {
		return fmt.Errorf("failed to convert total_recipients_opened: %w", err)
	}
	if s.TotalRecipientsClicked, err = convertToInt(tmp.TotalRecipientsClicked); err != nil {
		return fmt.Errorf("failed to convert total_recipients_clicked: %w", err)
	}
	if s.TotalDeliveryErrors, err = convertToInt(tmp.TotalDeliveryErrors); err != nil {
		return fmt.Errorf("failed to convert total_delivery_errors: %w", err)
	}

	return nil
}

// SurveyCommitteeDBRaw represents raw committee data from v1 DynamoDB
type SurveyCommitteeDBRaw struct {
	CommitteeID            string `json:"committee_id"` // v1 SFID
	CommitteeName          string `json:"committee_name"`
	ProjectID              string `json:"project_id"` // v1 SFID
	ProjectName            string `json:"project_name"`
	NPSValue               int    `json:"nps_value"`
	NumPromoters           int    `json:"num_promoters"`
	NumPassives            int    `json:"num_passives"`
	NumDetractors          int    `json:"num_detractors"`
	TotalRecipients        int    `json:"total_recipients"`
	TotalSentRecipients    int    `json:"total_recipients_sent"`
	TotalResponses         int    `json:"total_responses"`
	TotalRecipientsOpened  int    `json:"total_recipients_opened"`
	TotalRecipientsClicked int    `json:"total_recipients_clicked"`
	TotalDeliveryErrors    int    `json:"total_delivery_errors"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
func (c *SurveyCommitteeDBRaw) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with interface{} types for numeric fields
	tmp := struct {
		CommitteeID            string      `json:"committee_id"`
		CommitteeName          string      `json:"committee_name"`
		ProjectID              string      `json:"project_id"`
		ProjectName            string      `json:"project_name"`
		NPSValue               interface{} `json:"nps_value"`
		NumPromoters           interface{} `json:"num_promoters"`
		NumPassives            interface{} `json:"num_passives"`
		NumDetractors          interface{} `json:"num_detractors"`
		TotalRecipients        interface{} `json:"total_recipients"`
		TotalSentRecipients    interface{} `json:"total_recipients_sent"`
		TotalResponses         interface{} `json:"total_responses"`
		TotalRecipientsOpened  interface{} `json:"total_recipients_opened"`
		TotalRecipientsClicked interface{} `json:"total_recipients_clicked"`
		TotalDeliveryErrors    interface{} `json:"total_delivery_errors"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Helper function to convert interface{} to int
	convertToInt := func(v interface{}) (int, error) {
		if v == nil {
			return 0, nil
		}
		switch val := v.(type) {
		case string:
			if val == "" {
				return 0, nil
			}
			return strconv.Atoi(val)
		case float64:
			return int(val), nil
		case int:
			return val, nil
		default:
			return 0, fmt.Errorf("invalid type for numeric field: %T", v)
		}
	}

	// Assign string fields directly
	c.CommitteeID = tmp.CommitteeID
	c.CommitteeName = tmp.CommitteeName
	c.ProjectID = tmp.ProjectID
	c.ProjectName = tmp.ProjectName

	// Convert numeric fields
	var err error
	if c.NPSValue, err = convertToInt(tmp.NPSValue); err != nil {
		return fmt.Errorf("failed to convert nps_value: %w", err)
	}
	if c.NumPromoters, err = convertToInt(tmp.NumPromoters); err != nil {
		return fmt.Errorf("failed to convert num_promoters: %w", err)
	}
	if c.NumPassives, err = convertToInt(tmp.NumPassives); err != nil {
		return fmt.Errorf("failed to convert num_passives: %w", err)
	}
	if c.NumDetractors, err = convertToInt(tmp.NumDetractors); err != nil {
		return fmt.Errorf("failed to convert num_detractors: %w", err)
	}
	if c.TotalRecipients, err = convertToInt(tmp.TotalRecipients); err != nil {
		return fmt.Errorf("failed to convert total_recipients: %w", err)
	}
	if c.TotalSentRecipients, err = convertToInt(tmp.TotalSentRecipients); err != nil {
		return fmt.Errorf("failed to convert total_recipients_sent: %w", err)
	}
	if c.TotalResponses, err = convertToInt(tmp.TotalResponses); err != nil {
		return fmt.Errorf("failed to convert total_responses: %w", err)
	}
	if c.TotalRecipientsOpened, err = convertToInt(tmp.TotalRecipientsOpened); err != nil {
		return fmt.Errorf("failed to convert total_recipients_opened: %w", err)
	}
	if c.TotalRecipientsClicked, err = convertToInt(tmp.TotalRecipientsClicked); err != nil {
		return fmt.Errorf("failed to convert total_recipients_clicked: %w", err)
	}
	if c.TotalDeliveryErrors, err = convertToInt(tmp.TotalDeliveryErrors); err != nil {
		return fmt.Errorf("failed to convert total_delivery_errors: %w", err)
	}

	return nil
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

	// Build v2 survey data struct - numeric fields are now properly typed after UnmarshalJSON
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
		SurveyReminderRateDays: surveyDB.SurveyReminderRateDays,
		SendImmediately:        surveyDB.SendImmediately,
		EmailSubject:           surveyDB.EmailSubject,
		EmailBody:              surveyDB.EmailBody,
		EmailBodyText:          surveyDB.EmailBodyText,
		CommitteeCategory:      surveyDB.CommitteeCategory,
		CommitteeVotingEnabled: surveyDB.CommitteeVotingEnabled,
		SurveyStatus:           surveyDB.SurveyStatus,
		NPSValue:               surveyDB.NPSValue,
		NumPromoters:           surveyDB.NumPromoters,
		NumPassives:            surveyDB.NumPassives,
		NumDetractors:          surveyDB.NumDetractors,
		TotalRecipients:        surveyDB.TotalRecipients,
		TotalSentRecipients:    surveyDB.TotalSentRecipients,
		TotalResponses:         surveyDB.TotalResponses,
		TotalRecipientsOpened:  surveyDB.TotalRecipientsOpened,
		TotalRecipientsClicked: surveyDB.TotalRecipientsClicked,
		TotalDeliveryErrors:    surveyDB.TotalDeliveryErrors,
		IsNPSSurvey:            surveyDB.IsNPSSurvey,
		CollectorURL:           surveyDB.CollectorURL,
	}

	// Process committees array - numeric fields are now properly typed after UnmarshalJSON
	for _, committeeDB := range surveyDB.Committees {
		committeeData := domain.SurveyCommitteeData{
			CommitteeID:            committeeDB.CommitteeID,
			CommitteeName:          committeeDB.CommitteeName,
			ProjectID:              committeeDB.ProjectID,
			ProjectName:            committeeDB.ProjectName,
			NPSValue:               committeeDB.NPSValue,
			NumPromoters:           committeeDB.NumPromoters,
			NumPassives:            committeeDB.NumPassives,
			NumDetractors:          committeeDB.NumDetractors,
			TotalRecipients:        committeeDB.TotalRecipients,
			TotalSentRecipients:    committeeDB.TotalSentRecipients,
			TotalResponses:         committeeDB.TotalResponses,
			TotalRecipientsOpened:  committeeDB.TotalRecipientsOpened,
			TotalRecipientsClicked: committeeDB.TotalRecipientsClicked,
			TotalDeliveryErrors:    committeeDB.TotalDeliveryErrors,
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
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "unavailable") || strings.Contains(errStr, "deadline") {
		return true
	}

	return false
}
