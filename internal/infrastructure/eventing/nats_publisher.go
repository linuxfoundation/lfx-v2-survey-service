// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	indexerTypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	infraNATS "github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/nats"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracer is safe to initialize at package level — otel.Tracer() returns a
// delegating tracer that forwards to whatever TracerProvider is registered at
// call time, so otel.SetTracerProvider() updates it regardless of init order.
var tracer = otel.Tracer("github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/eventing")

// NATS subject constants for survey operations
const (
	// IndexSurveySubject is the subject for survey indexing
	IndexSurveySubject = "lfx.index.survey"

	// IndexSurveyResponseSubject is the subject for survey response indexing
	IndexSurveyResponseSubject = "lfx.index.survey_response"

	// IndexSurveyTemplateSubject is the subject for survey template indexing
	IndexSurveyTemplateSubject = "lfx.index.survey_template"
)

const (
	// authLookupTimeout bounds the auth-service email_to_sub request/reply lookup.
	authLookupTimeout = 5 * time.Second
)

var lfxUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func isValidLFXUsername(username string) bool {
	return lfxUsernamePattern.MatchString(username)
}

// NATSPublisher implements the EventPublisher interface
type NATSPublisher struct {
	conn   *nats.Conn
	logger *slog.Logger
}

// Compile-time assertion: *NATSPublisher must satisfy domain.EventPublisher.
var _ domain.EventPublisher = (*NATSPublisher)(nil)

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
		if err := p.sendDeleteAccessMessage(ctx, "survey", survey.UID); err != nil {
			return fmt.Errorf("failed to send survey delete access message: %w", err)
		}
	} else {
		if err := p.sendSurveyAccessMessage(ctx, survey); err != nil {
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
		if err := p.sendDeleteAccessMessage(ctx, "survey_response", response.UID); err != nil {
			return fmt.Errorf("failed to send survey response delete access message: %w", err)
		}
	} else {
		if err := p.sendSurveyResponseAccessMessage(ctx, response); err != nil {
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

// publishWithSpan wraps conn.PublishMsg with an OTel producer span and injects
// trace context into the NATS message headers.
func (p *NATSPublisher) publishWithSpan(ctx context.Context, subject string, data []byte) error {
	ctx, span := tracer.Start(ctx, "nats.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.String("messaging.operation.type", "publish"),
			attribute.Int("messaging.message.body.size", len(data)),
		),
	)
	defer span.End()

	msg := nats.NewMsg(subject)
	msg.Header = make(nats.Header)
	msg.Data = data
	otel.GetTextMapPropagator().Inject(ctx, infraNATS.NatsHeaderCarrier(msg.Header))

	if err := p.conn.PublishMsg(msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to publish to subject %s: %w", subject, err)
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
func (p *NATSPublisher) sendSurveyAccessMessage(ctx context.Context, survey *domain.SurveyData) error {
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

	return p.publishWithSpan(ctx, fgaconstants.GenericUpdateAccessSubject, accessMsgBytes)
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
		AccessCheckObject:    fmt.Sprintf("survey_response:%s", data.UID),
		AccessCheckRelation:  "auditor",
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
func (p *NATSPublisher) sendSurveyResponseAccessMessage(ctx context.Context, data *domain.SurveyResponseData) error {
	relations := map[string][]string{}
	if data.Username != "" {
		if isValidLFXUsername(data.Username) {
			relations["owner"] = []string{data.Username}
		} else {
			p.logger.WarnContext(ctx, "skipping FGA owner relation for invalid LFX username",
				"survey_response_uid", data.UID,
				"username", data.Username,
			)
		}
	}

	// Invitations without a usable username (empty, or a legacy value that fails
	// LFX username validation) resolve the invitee's primary email to their
	// Auth0 sub instead, so the FGA owner tuple lands on their account and the
	// response surfaces in their Pending Actions / My Surveys.
	if _, hasOwner := relations["owner"]; !hasOwner && data.Email != "" {
		sub, err := p.lookupEmailToAuthSub(ctx, data.Email)
		if err != nil {
			return fmt.Errorf("failed to resolve invitee email to auth sub: %w", err)
		}
		if sub != "" {
			relations["owner"] = []string{sub}
		}
	}

	references := map[string][]string{}
	if data.SurveyUID != "" {
		references["survey"] = []string{data.SurveyUID}
	}

	// Skip sending access message if there are no relations or references.
	// Skipping also preserves any existing owner tuple: no sync, no deletion.
	if len(relations) == 0 && len(references) == 0 {
		return nil
	}

	accessData := fgatypes.GenericAccessData{
		UID:        data.UID,
		Public:     false,
		Relations:  relations,
		References: references,
	}

	// fga-sync's update_access is a destructive full sync: a relation absent from
	// the payload has its live tuples deleted. Whenever no owner could be
	// resolved, exclude the owner relation so a previously granted owner tuple
	// (e.g. from an earlier invitation) survives re-sends and edits.
	if _, hasOwner := relations["owner"]; !hasOwner {
		accessData.ExcludeRelations = []string{"owner"}
	}

	accessMsg := fgatypes.GenericFGAMessage{
		ObjectType: "survey_response",
		Operation:  "update_access",
		Data:       accessData,
	}

	accessMsgBytes, err := json.Marshal(accessMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal access message: %w", err)
	}

	return p.publishWithSpan(ctx, fgaconstants.GenericUpdateAccessSubject, accessMsgBytes)
}

// lookupEmailToAuthSub resolves an email address to the user's Auth0 sub via the
// auth-service email_to_sub request/reply subject, following the same NATS
// request/reply pattern as idmapper.NATSMapper.lookup. A definitive resolution
// failure (unknown email) returns ("", nil) so the caller can degrade gracefully;
// transport failures (timeout, no responder) and transient auth-service backend
// failures (e.g. its Auth0 upstream) return an error so the event is retried
// instead of silently dropping the owner grant.
func (p *NATSPublisher) lookupEmailToAuthSub(ctx context.Context, email string) (string, error) {
	// auth-service lowercases and trims on its side too; send it canonical.
	payload := []byte(strings.ToLower(strings.TrimSpace(email)))

	ctx, span := tracer.Start(ctx, "nats.request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", surveyconstants.AuthEmailToSubSubject),
			attribute.Int("messaging.message.body.size", len(payload)),
		),
	)
	defer span.End()

	reqCtx, cancel := context.WithTimeout(ctx, authLookupTimeout)
	defer cancel()

	natsMsg := nats.NewMsg(surveyconstants.AuthEmailToSubSubject)
	natsMsg.Header = make(nats.Header)
	natsMsg.Data = payload
	otel.GetTextMapPropagator().Inject(reqCtx, infraNATS.NatsHeaderCarrier(natsMsg.Header))

	msg, err := p.conn.RequestMsgWithContext(reqCtx, natsMsg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("auth-service email_to_sub request failed: %w", err)
	}

	response := strings.TrimSpace(string(msg.Data))
	if response == "" {
		return "", nil
	}

	// auth-service reports resolution failures as JSON error envelopes —
	// handler-level {"success":false,"error":...} or transport-level
	// {"error":...} — while a success reply is the plain-text sub (never JSON).
	if strings.HasPrefix(response, "{") {
		var envelope struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal([]byte(response), &envelope); jsonErr != nil {
			p.logger.WarnContext(ctx, "unrecognized auth-service email_to_sub reply", "error", jsonErr)
			span.SetStatus(codes.Error, "unrecognized reply")
			return "", fmt.Errorf("%w: unrecognized email_to_sub reply: %v", domain.ErrAuthServiceLookupFailed, jsonErr)
		}
		if infraNATS.IsEmailLookupNotFound(envelope.Error) {
			// Definitive: no account owns this email. Degrade gracefully —
			// account-less invitees are an expected case (LFID invite flow).
			p.logger.DebugContext(ctx, "auth-service email_to_sub: no account for invitee email")
			span.SetStatus(codes.Ok, "email not resolvable")
			return "", nil
		}
		// Any other envelope error may be transient inside auth-service (e.g. its
		// Auth0 backend) — surface it so the KV event retries instead of silently
		// dropping the owner grant.
		p.logger.WarnContext(ctx, "auth-service email_to_sub lookup failed", "reason", envelope.Error)
		span.SetStatus(codes.Error, envelope.Error)
		return "", fmt.Errorf("%w: email_to_sub: %s", domain.ErrAuthServiceLookupFailed, envelope.Error)
	}

	span.SetStatus(codes.Ok, "")
	return response, nil
}

// sendDeleteAccessMessage sends a delete access message to FGA-sync
func (p *NATSPublisher) sendDeleteAccessMessage(ctx context.Context, objectType string, uid string) error {
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

	return p.publishWithSpan(ctx, fgaconstants.GenericDeleteAccessSubject, deleteMsgBytes)
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

	return p.publishWithSpan(ctx, subject, messageBytes)
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

	return p.publishWithSpan(ctx, subject, messageBytes)
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
