// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	indexerTypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go"
)

// NATS subject constants for survey operations
const (
	// IndexSurveySubject is the subject for survey indexing
	IndexSurveySubject = "lfx.index.survey"

	// IndexSurveyResponseSubject is the subject for survey response indexing
	IndexSurveyResponseSubject = "lfx.index.survey_response"

	// IndexSurveyTemplateSubject is the subject for survey template indexing
	IndexSurveyTemplateSubject = "lfx.index.survey_template"
)

// NATSPublisher implements the EventPublisher interface
type NATSPublisher struct {
	conn   *nats.Conn
	logger *slog.Logger
}

// NewNATSPublisher creates a new NATS publisher
func NewNATSPublisher(conn *nats.Conn, logger *slog.Logger) *NATSPublisher {
	return &NATSPublisher{
		conn:   conn,
		logger: logger,
	}
}

// PublishSurveyEvent publishes a survey event to indexer and FGA-sync
func (p *NATSPublisher) PublishSurveyEvent(ctx context.Context, action string, survey *domain.SurveyData) error {
	// Send to indexer
	if err := p.sendSurveyIndexerMessage(ctx, IndexSurveySubject, indexerConstants.MessageAction(action), survey); err != nil {
		return fmt.Errorf("failed to send survey indexer message: %w", err)
	}

	// Send to FGA-sync - different message for delete vs create/update
	if action == string(indexerConstants.ActionDeleted) {
		if err := p.sendDeleteAccessMessage("survey", survey.UID); err != nil {
			return fmt.Errorf("failed to send survey delete access message: %w", err)
		}
	} else {
		if err := p.sendSurveyAccessMessage(survey); err != nil {
			return fmt.Errorf("failed to send survey access message: %w", err)
		}
	}

	return nil
}

// PublishSurveyResponseEvent publishes a survey response event to indexer and FGA-sync
func (p *NATSPublisher) PublishSurveyResponseEvent(ctx context.Context, action string, response *domain.SurveyResponseData) error {
	// Send to indexer
	if err := p.sendSurveyResponseIndexerMessage(ctx, IndexSurveyResponseSubject, indexerConstants.MessageAction(action), response); err != nil {
		return fmt.Errorf("failed to send survey response indexer message: %w", err)
	}

	// Send to FGA-sync - different message for delete vs create/update
	if action == string(indexerConstants.ActionDeleted) {
		if err := p.sendDeleteAccessMessage("survey_response", response.UID); err != nil {
			return fmt.Errorf("failed to send survey response delete access message: %w", err)
		}
	} else {
		if err := p.sendSurveyResponseAccessMessage(response); err != nil {
			return fmt.Errorf("failed to send survey response access message: %w", err)
		}
	}

	return nil
}

// PublishSurveyTemplateEvent publishes a survey template event to the indexer
func (p *NATSPublisher) PublishSurveyTemplateEvent(ctx context.Context, action string, template *domain.SurveyTemplateData) error {
	if err := p.sendSurveyTemplateIndexerMessage(ctx, IndexSurveyTemplateSubject, indexerConstants.MessageAction(action), template); err != nil {
		return fmt.Errorf("failed to send survey template indexer message: %w", err)
	}
	return nil
}

// Close closes the publisher connection
func (p *NATSPublisher) Close() error {
	// NATS connection is managed by the event processor, so we don't close it here
	return nil
}

// appendIfNotExists adds a value to a slice only if it doesn't already exist
func appendIfNotExists(slice []string, value string) []string {
	if !slices.Contains(slice, value) {
		return append(slice, value)
	}
	return slice
}

// sendSurveyIndexerMessage routes to the appropriate indexer message handler based on action
func (p *NATSPublisher) sendSurveyIndexerMessage(ctx context.Context, subject string, action indexerConstants.MessageAction, data *domain.SurveyData) error {
	// Build IndexingConfig (needed for both create/update and delete)
	nameAndAliases := []string{}
	parentRefs := []string{}
	tags := []string{}

	if data.SurveyTitle != "" {
		nameAndAliases = append(nameAndAliases, data.SurveyTitle)
	}

	// Add committee and project references from committees array
	for _, committee := range data.Committees {
		if committee.CommitteeUID != "" {
			parentRefs = append(parentRefs, fmt.Sprintf("committee:%s", committee.CommitteeUID))
			tags = append(tags, fmt.Sprintf("committee_uid:%s", committee.CommitteeUID))
		}
		if committee.ProjectUID != "" {
			parentRefs = appendIfNotExists(parentRefs, fmt.Sprintf("project:%s", committee.ProjectUID))
			tags = appendIfNotExists(tags, fmt.Sprintf("project_uid:%s", committee.ProjectUID))
		}
	}

	indexingConfig := &indexerTypes.IndexingConfig{
		ObjectID:             data.UID,
		AccessCheckObject:    fmt.Sprintf("survey:%s", data.UID),
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   fmt.Sprintf("survey:%s", data.UID),
		HistoryCheckRelation: "auditor",
		SortName:             data.SurveyTitle,
		NameAndAliases:       nameAndAliases,
		ParentRefs:           parentRefs,
		Tags:                 tags,
		Fulltext:             data.SurveyTitle,
	}

	if action == indexerConstants.ActionDeleted {
		return p.sendIndexerDeleteMessage(ctx, subject, action, data.UID, indexingConfig)
	}

	return p.sendIndexerCreateUpdateMessage(ctx, subject, action, data, indexingConfig)
}

// sendSurveyAccessMessage sends the message to the NATS server for the survey access control
func (p *NATSPublisher) sendSurveyAccessMessage(survey *domain.SurveyData) error {
	// Build committee and project references
	committeeRefs := []string{}
	projectRefs := []string{}

	for _, committee := range survey.Committees {
		if committee.CommitteeUID != "" {
			committeeRefs = append(committeeRefs, committee.CommitteeUID)
		}
		if committee.ProjectUID != "" {
			projectRefs = appendIfNotExists(projectRefs, committee.ProjectUID)
		}
	}

	references := map[string][]string{}
	if len(committeeRefs) > 0 {
		references["committee"] = committeeRefs
	}
	if len(projectRefs) > 0 {
		references["project"] = projectRefs
	}

	// Skip sending access message if there are no references
	if len(references) == 0 {
		return nil
	}

	accessMsg := fgatypes.GenericFGAMessage{
		ObjectType: "survey",
		Operation:  "update_access",
		Data: fgatypes.GenericAccessData{
			UID:        survey.UID,
			Public:     false,
			References: references,
		},
	}

	accessMsgBytes, err := json.Marshal(accessMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal access message: %w", err)
	}

	// Publish the message to NATS
	if err := p.conn.Publish(fgaconstants.GenericUpdateAccessSubject, accessMsgBytes); err != nil {
		return fmt.Errorf("failed to publish access message to subject %s: %w", fgaconstants.GenericUpdateAccessSubject, err)
	}

	return nil
}

// sendSurveyResponseIndexerMessage routes to the appropriate indexer message handler based on action
func (p *NATSPublisher) sendSurveyResponseIndexerMessage(ctx context.Context, subject string, action indexerConstants.MessageAction, data *domain.SurveyResponseData) error {
	// Build IndexingConfig (needed for both create/update and delete)
	nameAndAliases := []string{}
	parentRefs := []string{}
	tags := []string{}

	if data.Email != "" {
		nameAndAliases = append(nameAndAliases, data.Email)
	}
	if data.Project.ProjectUID != "" {
		parentRefs = append(parentRefs, fmt.Sprintf("project:%s", data.Project.ProjectUID))
		tags = append(tags, fmt.Sprintf("project_uid:%s", data.Project.ProjectUID))
	}
	if data.CommitteeUID != "" {
		parentRefs = append(parentRefs, fmt.Sprintf("committee:%s", data.CommitteeUID))
		tags = append(tags, fmt.Sprintf("committee_uid:%s", data.CommitteeUID))
	}
	if data.SurveyUID != "" {
		parentRefs = append(parentRefs, fmt.Sprintf("survey:%s", data.SurveyUID))
		tags = append(tags, fmt.Sprintf("survey_uid:%s", data.SurveyUID))
	}

	indexingConfig := &indexerTypes.IndexingConfig{
		ObjectID:             data.UID,
		AccessCheckObject:    fmt.Sprintf("survey:%s", data.SurveyUID),
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   fmt.Sprintf("survey_response:%s", data.UID),
		HistoryCheckRelation: "auditor",
		SortName:             data.Email,
		NameAndAliases:       nameAndAliases,
		ParentRefs:           parentRefs,
		Tags:                 tags,
		Fulltext:             fmt.Sprintf("%s %s %s", data.Email, data.FirstName, data.LastName),
	}

	if action == indexerConstants.ActionDeleted {
		return p.sendIndexerDeleteMessage(ctx, subject, action, data.UID, indexingConfig)
	}

	return p.sendIndexerCreateUpdateMessage(ctx, subject, action, data, indexingConfig)
}

// sendSurveyTemplateIndexerMessage routes to the appropriate indexer message handler based on action
func (p *NATSPublisher) sendSurveyTemplateIndexerMessage(ctx context.Context, subject string, action indexerConstants.MessageAction, data *domain.SurveyTemplateData) error {
	nameAndAliases := []string{}
	if data.Title != "" {
		nameAndAliases = append(nameAndAliases, data.Title)
	}
	if data.Nickname != "" {
		nameAndAliases = appendIfNotExists(nameAndAliases, data.Nickname)
	}

	indexingConfig := &indexerTypes.IndexingConfig{
		ObjectID:             data.ID,
		AccessCheckObject:    "team:global_survey_platform_admins",
		AccessCheckRelation:  "member",
		HistoryCheckObject:   "team:global_survey_platform_admins",
		HistoryCheckRelation: "member",
		SortName:             data.Title,
		NameAndAliases:       nameAndAliases,
		Fulltext:             fmt.Sprintf("%s %s", data.Title, data.Nickname),
	}

	if action == indexerConstants.ActionDeleted {
		return p.sendIndexerDeleteMessage(ctx, subject, action, data.ID, indexingConfig)
	}

	return p.sendIndexerCreateUpdateMessage(ctx, subject, action, data, indexingConfig)
}

// sendSurveyResponseAccessMessage sends the message to the NATS server for the survey response access control
func (p *NATSPublisher) sendSurveyResponseAccessMessage(data *domain.SurveyResponseData) error {
	relations := map[string][]string{}
	if data.Username != "" {
		relations["owner"] = []string{data.Username}
	}

	references := map[string][]string{}
	if data.SurveyUID != "" {
		references["survey"] = []string{data.SurveyUID}
	}

	// Skip sending access message if there are no relations or references
	if len(relations) == 0 && len(references) == 0 {
		return nil
	}

	accessMsg := fgatypes.GenericFGAMessage{
		ObjectType: "survey_response",
		Operation:  "update_access",
		Data: fgatypes.GenericAccessData{
			UID:        data.UID,
			Public:     false,
			Relations:  relations,
			References: references,
		},
	}

	accessMsgBytes, err := json.Marshal(accessMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal access message: %w", err)
	}

	// Publish the message to NATS
	if err := p.conn.Publish(fgaconstants.GenericUpdateAccessSubject, accessMsgBytes); err != nil {
		return fmt.Errorf("failed to publish access message to subject %s: %w", fgaconstants.GenericUpdateAccessSubject, err)
	}

	return nil
}

// sendDeleteAccessMessage sends a delete access message to FGA-sync
func (p *NATSPublisher) sendDeleteAccessMessage(objectType string, uid string) error {
	// Construct delete access message
	deleteMsg := fgatypes.GenericFGAMessage{
		ObjectType: objectType,
		Operation:  "delete_access",
		Data:       fgatypes.GenericDeleteData{UID: uid},
	}

	deleteMsgBytes, err := json.Marshal(deleteMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal delete access message: %w", err)
	}

	// Publish the message to NATS
	if err := p.conn.Publish(fgaconstants.GenericDeleteAccessSubject, deleteMsgBytes); err != nil {
		return fmt.Errorf("failed to publish delete access message to subject %s: %w", fgaconstants.GenericDeleteAccessSubject, err)
	}

	return nil
}

// sendIndexerDeleteMessage sends a generic delete message to the indexer with just the UID
func (p *NATSPublisher) sendIndexerDeleteMessage(ctx context.Context, subject string, action indexerConstants.MessageAction, uid string, indexingConfig *indexerTypes.IndexingConfig) error {
	headers := p.buildHeaders(ctx)

	message := indexerTypes.IndexerMessageEnvelope{
		Action:         action,
		Headers:        headers,
		Data:           uid,
		IndexingConfig: indexingConfig,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal indexer delete message for subject %s: %w", subject, err)
	}

	p.logger.With("subject", subject, "action", action, "uid", uid).DebugContext(ctx, "constructed indexer delete message")

	// Publish the message to NATS
	if err := p.conn.Publish(subject, messageBytes); err != nil {
		return fmt.Errorf("failed to publish indexer delete message to subject %s: %w", subject, err)
	}

	return nil
}

// sendIndexerCreateUpdateMessage sends a generic create/update message to the indexer with full object and IndexingConfig
func (p *NATSPublisher) sendIndexerCreateUpdateMessage(ctx context.Context, subject string, action indexerConstants.MessageAction, data interface{}, indexingConfig *indexerTypes.IndexingConfig) error {
	headers := p.buildHeaders(ctx)

	public := false
	indexingConfig.Public = &public

	// Construct the indexer message
	message := indexerTypes.IndexerMessageEnvelope{
		Action:         action,
		Headers:        headers,
		Data:           data,
		IndexingConfig: indexingConfig,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal indexer message for subject %s: %w", subject, err)
	}

	p.logger.With("subject", subject, "action", action).DebugContext(ctx, "constructed indexer message")

	// Publish the message to NATS
	if err := p.conn.Publish(subject, messageBytes); err != nil {
		return fmt.Errorf("failed to publish indexer message to subject %s: %w", subject, err)
	}

	return nil
}

// buildHeaders extracts headers from context for NATS messages
func (p *NATSPublisher) buildHeaders(ctx context.Context) map[string]string {
	headers := make(map[string]string)

	// Extract authorization from context if available
	if authorization, ok := ctx.Value("authorization").(string); ok {
		headers["authorization"] = authorization
	} else {
		// Fallback for system-generated events
		headers["authorization"] = "Bearer survey-service"
	}

	// Extract principal from context if available
	if principal, ok := ctx.Value("principal").(string); ok {
		headers["x-on-behalf-of"] = principal
	}

	return headers
}
