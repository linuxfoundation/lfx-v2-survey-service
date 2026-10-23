// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// UserReader looks up LFID user data by email.
type UserReader interface {
	UsernameByEmail(ctx context.Context, email string) (string, error)
}

// UserNameReader resolves the display name of an LFID user from a trusted source.
type UserNameReader interface {
	// DisplayNameByUsername returns the user's full name, or ErrUserNotFound when
	// the account is unknown or has no name on record.
	DisplayNameByUsername(ctx context.Context, username string) (string, error)
}
