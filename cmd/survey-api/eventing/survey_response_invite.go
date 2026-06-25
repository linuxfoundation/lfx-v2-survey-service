// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

// SurveyResponseInviteHandler performs best-effort LFID invite sending for new survey responses.
type SurveyResponseInviteHandler struct {
	inviteSender     domain.InviteSender
	userReader       domain.UserReader
	v1ObjectsKV      jetstream.KeyValue
	v1MappingsKV     jetstream.KeyValue
	selfServeBaseURL string
}

// inviteEnabled reports whether outbound invite sending is fully wired up.
func (h *SurveyResponseInviteHandler) inviteEnabled() bool {
	return h != nil &&
		h.inviteSender != nil &&
		h.userReader != nil &&
		strings.TrimSpace(h.selfServeBaseURL) != ""
}

const surveyResponseLFIDInviteSentKeyFmt = "v1_survey_response_lfid_invite_sent.%s"

func surveyResponseLFIDInviteSentKey(surveyResponseUID string) string {
	return fmt.Sprintf(surveyResponseLFIDInviteSentKeyFmt, surveyResponseUID)
}

// maybeSendInvite performs a best-effort LFID invite for a new survey-response participant
// who has no username. All errors are logged and swallowed.
func (h *SurveyResponseInviteHandler) maybeSendInvite(
	ctx context.Context,
	logger *slog.Logger,
	surveyResponseUID, email, displayName, surveyID string,
) {
	if !h.inviteEnabled() {
		return
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return
	}

	inviteSentKey := surveyResponseLFIDInviteSentKey(surveyResponseUID)
	if _, err := h.v1MappingsKV.Get(ctx, inviteSentKey); err == nil {
		logger.DebugContext(ctx, "LFID invite already sent for survey response, skipping")
		return
	} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
		logger.With(errKey, err).WarnContext(ctx, "failed to check LFID invite sent marker; skipping invite")
		return
	}

	username, err := h.userReader.UsernameByEmail(ctx, email)
	if err == nil && username != "" {
		logger.DebugContext(ctx, "survey response participant already has LFID, skipping invite")
		return
	}
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		// Transient auth-service error: fall through and attempt the invite anyway.
		// Skipping here would permanently lose the invite opportunity — the KV mapping
		// is already stored so this message won't be redelivered as ActionCreated.
		// The invite service handles the edge case where the user already has an LFID.
		logger.With(errKey, err).WarnContext(ctx, "failed to check LFID for survey response; proceeding with invite as best-effort")
	}

	var surveyName string
	surveyKey := fmt.Sprintf("itx-surveys.%s", surveyID)
	if entry, kvErr := h.v1ObjectsKV.Get(ctx, surveyKey); kvErr == nil {
		if data, decErr := decodeKVValue(entry.Value()); decErr == nil {
			if name, ok := data["name"].(string); ok {
				surveyName = strings.TrimSpace(name)
			}
		}
	}
	if surveyName == "" {
		logger.WarnContext(ctx, "could not resolve survey name; skipping invite to avoid confusing email")
		return
	}

	returnURL := fmt.Sprintf("%s/surveys/%s", strings.TrimRight(h.selfServeBaseURL, "/"), url.PathEscape(surveyID))
	name := strings.TrimSpace(displayName)
	req := inviteapi.SendInviteRequest{
		Recipient: &inviteapi.Recipient{
			Email: email,
			Name:  name,
		},
		Resource: &inviteapi.Resource{
			UID:  surveyID,
			Name: surveyName,
			Type: surveyconstants.ResourceTypeSurvey,
		},
		Role:           surveyconstants.InviteRoleParticipant,
		ReturnURL:      returnURL,
		ExpirationDays: 30,
	}

	// Write a "pending" marker before calling SendInvite to close the duplicate-invite
	// window: a concurrent redelivery that passes the Get check above would also see
	// this marker and skip, preventing two goroutines from both calling SendInvite.
	if _, err := h.v1MappingsKV.Put(ctx, inviteSentKey, []byte("pending")); err != nil {
		logger.With(errKey, err).WarnContext(ctx, "failed to store pending invite marker; skipping to avoid duplicate")
		return
	}

	result, sendErr := h.inviteSender.SendInvite(ctx, req)
	if sendErr != nil {
		logger.With(errKey, sendErr).WarnContext(ctx, "failed to send LFID invite for survey response; continuing")
		return
	}
	if _, err := h.v1MappingsKV.Put(ctx, inviteSentKey, []byte(result.InviteUID)); err != nil {
		logger.With(errKey, err).WarnContext(ctx, "failed to update survey response LFID invite sent marker")
	}
	logger.InfoContext(ctx, "sent LFID invite for survey response",
		"invite_uid", result.InviteUID,
		"expires_at", result.ExpiresAt,
	)
}

// shouldSendSurveyResponseInvite reports whether a new no-LFID survey response should trigger an invite.
func shouldSendSurveyResponseInvite(indexerAction indexerConstants.MessageAction, username, email string) bool {
	return indexerAction == indexerConstants.ActionCreated &&
		strings.TrimSpace(username) == "" &&
		strings.TrimSpace(email) != ""
}
