// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"time"
)

// InviteRecipient identifies the person being invited.
type InviteRecipient struct {
	Email string
	Name  string
}

// InviteResource describes the resource the invite is for.
type InviteResource struct {
	UID  string
	Name string
	Type string
}

// InviteRequest is the domain-native representation of a request to send an LFID invite.
// It is intentionally free of any external-service wire types; the infrastructure adapter
// is responsible for translating this into the invite-service's wire format.
type InviteRequest struct {
	Recipient      InviteRecipient
	Resource       InviteResource
	Role           string
	ReturnURL      string
	ExpirationDays int
}

// InviteResult holds the key fields returned by the invite service after an invite is sent.
type InviteResult struct {
	InviteUID      string
	RecipientEmail string
	ExpiresAt      time.Time
}

// InviteSender sends LFID invites via the invite service over NATS.
type InviteSender interface {
	SendInvite(ctx context.Context, req InviteRequest) (*InviteResult, error)
}
