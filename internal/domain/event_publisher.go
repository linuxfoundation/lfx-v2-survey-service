// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// EventPublisher defines the interface for publishing survey and survey response events
// to the indexer and FGA-sync services
type EventPublisher interface {
	// PublishSurveyEvent publishes a survey event to indexer and FGA-sync
	// action should be "created", "updated", or "deleted"
	PublishSurveyEvent(ctx context.Context, action string, survey *SurveyData) error

	// PublishSurveyResponseEvent publishes a survey response event to indexer and FGA-sync
	// action should be "created", "updated", or "deleted"
	PublishSurveyResponseEvent(ctx context.Context, action string, response *SurveyResponseData) error

	// Close closes the publisher connection
	Close() error
}
