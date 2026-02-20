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
	"github.com/nats-io/nats.go/jetstream"
)

// SurveyResponseDBRaw represents raw survey response data from v1 DynamoDB/NATS KV bucket
type SurveyResponseDBRaw struct {
	ID                            string                               `json:"id"`
	SurveyID                      string                               `json:"survey_id"`
	SurveyMonkeyRespondent        string                               `json:"survey_monkey_respondent_id"`
	Email                         string                               `json:"email"`
	CommitteeMemberID             string                               `json:"committee_member_id,omitempty"`
	FirstName                     string                               `json:"first_name"`
	LastName                      string                               `json:"last_name"`
	CreatedAt                     string                               `json:"created_at"`
	ResponseDatetime              string                               `json:"response_datetime"`
	LastReceivedTime              string                               `json:"last_received_time"`
	NumAutomatedRemindersReceived int                                  `json:"num_automated_reminders_received"`
	Username                      string                               `json:"username"`
	VotingStatus                  string                               `json:"voting_status"`
	Role                          string                               `json:"role"`
	JobTitle                      string                               `json:"job_title"`
	MembershipTier                string                               `json:"membership_tier"`
	Organization                  domain.SurveyResponseOrgData         `json:"organization"`
	Project                       SurveyResponseProjectDBRaw           `json:"project"`
	CommitteeID                   string                               `json:"committee_id"` // v1 SFID
	CommitteeVotingEnabled        bool                                 `json:"committee_voting_enabled"`
	SurveyLink                    string                               `json:"survey_link"`
	NPSValue                      int                                  `json:"nps_value"`
	SurveyMonkeyQuestionAnswers   []domain.SurveyMonkeyQuestionAnswers `json:"survey_monkey_question_answers"`
	SESMessageID                  string                               `json:"ses_message_id"`
	SESBounceType                 string                               `json:"ses_bounce_type"`
	SESBounceSubtype              string                               `json:"ses_bounce_subtype"`
	SESBounceDiagnosticCode       string                               `json:"ses_bounce_diagnostic_code"`
	SESComplaintExists            bool                                 `json:"ses_complaint_exists"`
	SESComplaintType              string                               `json:"ses_complaint_type"`
	SESComplaintDate              string                               `json:"ses_complaint_date"`
	SESDeliverySuccessful         bool                                 `json:"ses_delivery_successful"`
	EmailOpenedFirstTime          string                               `json:"email_opened_first_time"`
	EmailOpenedLastTime           string                               `json:"email_opened_last_time"`
	LinkClickedFirstTime          string                               `json:"link_clicked_first_time"`
	LinkClickedLastTime           string                               `json:"link_clicked_last_time"`
	Excluded                      bool                                 `json:"excluded"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
func (r *SurveyResponseDBRaw) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with interface{} types for numeric fields
	tmp := struct {
		ID                            string                               `json:"id"`
		SurveyID                      string                               `json:"survey_id"`
		SurveyMonkeyRespondent        string                               `json:"survey_monkey_respondent_id"`
		Email                         string                               `json:"email"`
		CommitteeMemberID             string                               `json:"committee_member_id,omitempty"`
		FirstName                     string                               `json:"first_name"`
		LastName                      string                               `json:"last_name"`
		CreatedAt                     string                               `json:"created_at"`
		ResponseDatetime              string                               `json:"response_datetime"`
		LastReceivedTime              string                               `json:"last_received_time"`
		NumAutomatedRemindersReceived interface{}                          `json:"num_automated_reminders_received"`
		Username                      string                               `json:"username"`
		VotingStatus                  string                               `json:"voting_status"`
		Role                          string                               `json:"role"`
		JobTitle                      string                               `json:"job_title"`
		MembershipTier                string                               `json:"membership_tier"`
		Organization                  domain.SurveyResponseOrgData         `json:"organization"`
		Project                       SurveyResponseProjectDBRaw           `json:"project"`
		CommitteeID                   string                               `json:"committee_id"`
		CommitteeVotingEnabled        bool                                 `json:"committee_voting_enabled"`
		SurveyLink                    string                               `json:"survey_link"`
		NPSValue                      interface{}                          `json:"nps_value"`
		SurveyMonkeyQuestionAnswers   []domain.SurveyMonkeyQuestionAnswers `json:"survey_monkey_question_answers"`
		SESMessageID                  string                               `json:"ses_message_id"`
		SESBounceType                 string                               `json:"ses_bounce_type"`
		SESBounceSubtype              string                               `json:"ses_bounce_subtype"`
		SESBounceDiagnosticCode       string                               `json:"ses_bounce_diagnostic_code"`
		SESComplaintExists            bool                                 `json:"ses_complaint_exists"`
		SESComplaintType              string                               `json:"ses_complaint_type"`
		SESComplaintDate              string                               `json:"ses_complaint_date"`
		SESDeliverySuccessful         bool                                 `json:"ses_delivery_successful"`
		EmailOpenedFirstTime          string                               `json:"email_opened_first_time"`
		EmailOpenedLastTime           string                               `json:"email_opened_last_time"`
		LinkClickedFirstTime          string                               `json:"link_clicked_first_time"`
		LinkClickedLastTime           string                               `json:"link_clicked_last_time"`
		Excluded                      bool                                 `json:"excluded"`
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

	// Assign all fields
	r.ID = tmp.ID
	r.SurveyID = tmp.SurveyID
	r.SurveyMonkeyRespondent = tmp.SurveyMonkeyRespondent
	r.Email = tmp.Email
	r.CommitteeMemberID = tmp.CommitteeMemberID
	r.FirstName = tmp.FirstName
	r.LastName = tmp.LastName
	r.CreatedAt = tmp.CreatedAt
	r.ResponseDatetime = tmp.ResponseDatetime
	r.LastReceivedTime = tmp.LastReceivedTime
	r.Username = tmp.Username
	r.VotingStatus = tmp.VotingStatus
	r.Role = tmp.Role
	r.JobTitle = tmp.JobTitle
	r.MembershipTier = tmp.MembershipTier
	r.Organization = tmp.Organization
	r.Project = tmp.Project
	r.CommitteeID = tmp.CommitteeID
	r.CommitteeVotingEnabled = tmp.CommitteeVotingEnabled
	r.SurveyLink = tmp.SurveyLink
	r.SurveyMonkeyQuestionAnswers = tmp.SurveyMonkeyQuestionAnswers
	r.SESMessageID = tmp.SESMessageID
	r.SESBounceType = tmp.SESBounceType
	r.SESBounceSubtype = tmp.SESBounceSubtype
	r.SESBounceDiagnosticCode = tmp.SESBounceDiagnosticCode
	r.SESComplaintExists = tmp.SESComplaintExists
	r.SESComplaintType = tmp.SESComplaintType
	r.SESComplaintDate = tmp.SESComplaintDate
	r.SESDeliverySuccessful = tmp.SESDeliverySuccessful
	r.EmailOpenedFirstTime = tmp.EmailOpenedFirstTime
	r.EmailOpenedLastTime = tmp.EmailOpenedLastTime
	r.LinkClickedFirstTime = tmp.LinkClickedFirstTime
	r.LinkClickedLastTime = tmp.LinkClickedLastTime
	r.Excluded = tmp.Excluded

	// Convert numeric fields
	var err error
	if r.NumAutomatedRemindersReceived, err = convertToInt(tmp.NumAutomatedRemindersReceived); err != nil {
		return fmt.Errorf("failed to convert num_automated_reminders_received: %w", err)
	}
	if r.NPSValue, err = convertToInt(tmp.NPSValue); err != nil {
		return fmt.Errorf("failed to convert nps_value: %w", err)
	}

	return nil
}

// SurveyResponseProjectDBRaw represents raw project data from v1
type SurveyResponseProjectDBRaw struct {
	ID   string `json:"id"` // v1 SFID
	Name string `json:"name"`
}

// handleSurveyResponseUpdate processes a survey response update from itx-survey-responses records
// Returns true if the message should be retried (NAK), false if it should be acknowledged (ACK)
func handleSurveyResponseUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("key", key, "handler", "survey_response")

	funcLogger.DebugContext(ctx, "processing survey response update")

	// Convert v1Data map to survey response data with proper v2 format
	responseData, err := convertMapToSurveyResponseData(ctx, v1Data, idMapper, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to survey response")
		return false // Permanent error, ACK and skip
	}

	// Extract the survey response UID
	if responseData.UID == "" {
		funcLogger.ErrorContext(ctx, "missing or invalid uid in survey response data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("survey_response_id", responseData.UID)

	// Check if parent project exists in mappings
	if responseData.Project.ProjectUID == "" {
		funcLogger.With("project_id", responseData.Project.ID).InfoContext(ctx, "skipping survey response sync - parent project not found in mappings")
		return false // Permanent issue, ACK and skip
	}

	// Determine action (created vs updated) by checking if mapping exists
	mappingKey := fmt.Sprintf("survey_response.%s", responseData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := publisher.PublishSurveyResponseEvent(ctx, string(indexerAction), responseData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish survey response event")
		// Check if this is a transient error that should be retried
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping to track that we've seen this survey response
	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store survey response mapping")
		// Don't retry on mapping storage failures
	}

	funcLogger.InfoContext(ctx, "successfully sent survey response indexer and access messages")
	return false // Success, ACK the message
}

// convertMapToSurveyResponseData converts v1 survey response data to v2 format with proper types and UIDs
func convertMapToSurveyResponseData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	logger *slog.Logger,
) (*domain.SurveyResponseData, error) {
	// Convert map to JSON bytes, then to SurveyResponseDBRaw to handle string/raw fields
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var responseDB SurveyResponseDBRaw
	if err := json.Unmarshal(jsonBytes, &responseDB); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON into SurveyResponseDBRaw: %w", err)
	}

	// Build v2 survey response data struct - numeric fields are now properly typed after UnmarshalJSON
	responseData := &domain.SurveyResponseData{
		UID:                           responseDB.ID,
		ID:                            responseDB.ID,
		SurveyID:                      responseDB.SurveyID,
		SurveyUID:                     responseDB.SurveyID, // survey_id becomes survey_uid in v2
		SurveyMonkeyRespondent:        responseDB.SurveyMonkeyRespondent,
		Email:                         responseDB.Email,
		CommitteeMemberID:             responseDB.CommitteeMemberID,
		FirstName:                     responseDB.FirstName,
		LastName:                      responseDB.LastName,
		CreatedAt:                     responseDB.CreatedAt,
		ResponseDatetime:              responseDB.ResponseDatetime,
		LastReceivedTime:              responseDB.LastReceivedTime,
		NumAutomatedRemindersReceived: responseDB.NumAutomatedRemindersReceived,
		Username:                      responseDB.Username,
		VotingStatus:                  responseDB.VotingStatus,
		Role:                          responseDB.Role,
		JobTitle:                      responseDB.JobTitle,
		MembershipTier:                responseDB.MembershipTier,
		Organization:                  responseDB.Organization,
		CommitteeID:                   responseDB.CommitteeID,
		CommitteeVotingEnabled:        responseDB.CommitteeVotingEnabled,
		SurveyLink:                    responseDB.SurveyLink,
		NPSValue:                      responseDB.NPSValue,
		SurveyMonkeyQuestionAnswers:   responseDB.SurveyMonkeyQuestionAnswers,
		SESMessageID:                  responseDB.SESMessageID,
		SESBounceType:                 responseDB.SESBounceType,
		SESBounceSubtype:              responseDB.SESBounceSubtype,
		SESBounceDiagnosticCode:       responseDB.SESBounceDiagnosticCode,
		SESComplaintExists:            responseDB.SESComplaintExists,
		SESComplaintType:              responseDB.SESComplaintType,
		SESComplaintDate:              responseDB.SESComplaintDate,
		SESDeliverySuccessful:         responseDB.SESDeliverySuccessful,
		EmailOpenedFirstTime:          responseDB.EmailOpenedFirstTime,
		EmailOpenedLastTime:           responseDB.EmailOpenedLastTime,
		LinkClickedFirstTime:          responseDB.LinkClickedFirstTime,
		LinkClickedLastTime:           responseDB.LinkClickedLastTime,
		Excluded:                      responseDB.Excluded,
	}

	// Process project with ID mapping
	responseData.Project = domain.SurveyResponseProjectData{
		ID:   responseDB.Project.ID,
		Name: responseDB.Project.Name,
	}

	if responseDB.Project.ID != "" {
		projectUID, err := idMapper.MapProjectV1ToV2(ctx, responseDB.Project.ID)
		if err != nil {
			logger.With(errKey, err, "field", "project.id", "value", responseDB.Project.ID).
				WarnContext(ctx, "failed to get v2 project UID from v1 project ID")
			// Don't set project_uid if mapping fails - will be caught by validation
		} else {
			responseData.Project.ProjectUID = projectUID
		}
	}

	// Map v1 committee ID (SFID) to v2 committee UID
	if responseDB.CommitteeID != "" {
		committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, responseDB.CommitteeID)
		if err != nil {
			logger.With(errKey, err, "field", "committee_id", "value", responseDB.CommitteeID).
				WarnContext(ctx, "failed to get v2 committee UID from v1 committee ID")
			// Don't set committee_uid if mapping fails
		} else {
			responseData.CommitteeUID = committeeUID
		}
	}

	return responseData, nil
}

// handleSurveyResponseDelete processes a survey response delete from itx-survey-responses records
// Returns true if the message should be retried (NAK), false if it should be acknowledged (ACK)
func handleSurveyResponseDelete(
	ctx context.Context,
	uid string,
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("survey_response_uid", uid, "handler", "survey_response_delete")

	funcLogger.DebugContext(ctx, "processing survey response delete")

	// Create minimal survey response data for delete event
	responseData := &domain.SurveyResponseData{
		UID: uid,
		ID:  uid,
	}

	// Publish delete event to indexer and FGA-sync
	if err := publisher.PublishSurveyResponseEvent(ctx, string(indexerConstants.ActionDeleted), responseData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish survey response delete event")
		// Check if this is a transient error that should be retried
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping from v1-mappings KV
	mappingKey := fmt.Sprintf("survey_response.%s", uid)
	if err := mappingsKV.Delete(ctx, mappingKey); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to delete survey response mapping")
		// Don't retry on mapping deletion failures
	}

	funcLogger.InfoContext(ctx, "successfully sent survey response delete indexer and access messages")
	return false // Success, ACK the message
}
