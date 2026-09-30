// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
)

const userReaderTimeout = 10 * time.Second

// NATSUserReader implements domain.UserReader using NATS request/reply to the auth service.
type NATSUserReader struct {
	nc     Requester
	logger *slog.Logger
}

// NewUserReader creates a new NATS-based user reader.
func NewUserReader(nc Requester, logger *slog.Logger) *NATSUserReader {
	logger.Info("user reader initialized", "subject", surveyconstants.AuthEmailToUsernameSubject)
	return &NATSUserReader{nc: nc, logger: logger}
}

// UsernameByEmail returns the LFX username for the LFID account that owns the given email address.
func (r *NATSUserReader) UsernameByEmail(ctx context.Context, email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", domain.ErrUserNotFound
	}

	reqCtx, cancel := context.WithTimeout(ctx, userReaderTimeout)
	defer cancel()

	msg, err := r.nc.RequestWithContext(reqCtx, surveyconstants.AuthEmailToUsernameSubject, []byte(email))
	if err != nil {
		return "", fmt.Errorf("email_to_username request failed: %w", err)
	}

	body := strings.TrimSpace(string(msg.Data))
	if body == "" {
		return "", domain.ErrUserNotFound
	}

	if body[0] == '{' {
		var envelope struct {
			Success *bool  `json:"success"`
			Error   string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			return "", fmt.Errorf("failed to parse email_to_username response: %w", err)
		}
		if envelope.Success == nil {
			return "", fmt.Errorf("email_to_username response missing success field")
		}
		if !*envelope.Success {
			if errMsg := strings.TrimSpace(envelope.Error); errMsg != "" && !IsEmailLookupNotFound(errMsg) {
				return "", fmt.Errorf("email_to_username failed: %s", redactEmails(errMsg))
			}
			return "", domain.ErrUserNotFound
		}
		return "", fmt.Errorf("unexpected email_to_username success envelope: %s", body)
	}

	return body, nil
}

// IsEmailLookupNotFound reports whether an auth-service email lookup error message
// means "no account owns this email" (definitive) as opposed to a transient backend failure.
func IsEmailLookupNotFound(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "not found") || strings.Contains(lower, "no user")
}

var _ domain.UserReader = (*NATSUserReader)(nil)

// emailPattern matches plain and URL-escaped (%40) email addresses.
var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+-]+(?:@|%40)[A-Za-z0-9.-]+`)

// redactEmails strips email addresses from auth-service error text before it is
// wrapped into errors that callers log and retry (Auth0 transport failures embed
// the users-by-email request URL, which carries the invitee's address).
func redactEmails(s string) string {
	return emailPattern.ReplaceAllString(s, "[redacted-email]")
}
