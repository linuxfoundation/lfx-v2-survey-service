// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
)

// userMetadataReaderTimeout is short on purpose: the name stamped on a survey is
// best-effort and must not hold up scheduling on a slow auth-service responder.
const userMetadataReaderTimeout = 2 * time.Second

// NATSUserNameReader implements domain.UserNameReader using NATS request/reply
// to the auth service's user_metadata.read subject.
type NATSUserNameReader struct {
	nc     Requester
	logger *slog.Logger
}

// NewUserNameReader creates a new NATS-based user name reader.
func NewUserNameReader(nc Requester, logger *slog.Logger) *NATSUserNameReader {
	logger.Info("user name reader initialized", "subject", surveyconstants.AuthUserMetadataSubject)
	return &NATSUserNameReader{nc: nc, logger: logger}
}

// userMetadataResponse is the auth service reply envelope for user_metadata.read.
type userMetadataResponse struct {
	Success *bool  `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
	} `json:"data"`
}

// DisplayNameByUsername returns the full name of the LFX user with the given username.
func (r *NATSUserNameReader) DisplayNameByUsername(ctx context.Context, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", domain.ErrUserNotFound
	}

	reqCtx, cancel := context.WithTimeout(ctx, userMetadataReaderTimeout)
	defer cancel()

	msg, err := r.nc.RequestWithContext(reqCtx, surveyconstants.AuthUserMetadataSubject, []byte(username))
	if err != nil {
		return "", fmt.Errorf("user_metadata request failed: %w", err)
	}

	var meta userMetadataResponse
	if err := json.Unmarshal(msg.Data, &meta); err != nil {
		return "", fmt.Errorf("failed to parse user_metadata response: %w", err)
	}
	if meta.Success == nil {
		return "", fmt.Errorf("user_metadata response missing success field")
	}
	if !*meta.Success {
		return "", domain.ErrUserNotFound
	}

	name := strings.TrimSpace(meta.Data.Name)
	if name == "" {
		name = strings.TrimSpace(meta.Data.GivenName + " " + meta.Data.FamilyName)
	}
	if name == "" {
		return "", domain.ErrUserNotFound
	}
	return name, nil
}
