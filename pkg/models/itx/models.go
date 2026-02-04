// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import "time"

// ScheduleSurveyRequest represents the request to schedule a survey in ITX
type ScheduleSurveyRequest struct {
	IsProjectSurvey         *bool    `json:"is_project_survey,omitempty"`
	StageFilter             *string  `json:"stage_filter,omitempty"`
	CreatorUsername         *string  `json:"creator_username,omitempty"`
	CreatorName             *string  `json:"creator_name,omitempty"`
	CreatorID               *string  `json:"creator_id,omitempty"`
	SurveyMonkeyID          *string  `json:"survey_monkey_id,omitempty"`
	SurveyTitle             *string  `json:"survey_title,omitempty"`
	SendImmediately         *bool    `json:"send_immediately,omitempty"`
	SurveySendDate          *string  `json:"survey_send_date,omitempty"`      // RFC3339 string
	SurveyCutoffDate        *string  `json:"survey_cutoff_date,omitempty"`    // RFC3339 string
	SurveyReminderRateDays  *int     `json:"survey_reminder_rate_days,omitempty"`
	EmailSubject            *string  `json:"email_subject,omitempty"`
	EmailBody               *string  `json:"email_body,omitempty"`            // HTML
	EmailBodyText           *string  `json:"email_body_text,omitempty"`       // Plain text
	Committees              []string `json:"committees,omitempty"`
	CommitteeVotingEnabled  *bool    `json:"committee_voting_enabled,omitempty"`
}

// SurveyScheduleResponse represents the response from scheduling a survey
type SurveyScheduleResponse struct {
	ID                              string             `json:"id"`
	SurveyMonkeyID                  *string            `json:"survey_monkey_id,omitempty"`
	IsProjectSurvey                 *bool              `json:"is_project_survey,omitempty"`
	StageFilter                     *string            `json:"stage_filter,omitempty"`
	CreatorUsername                 *string            `json:"creator_username,omitempty"`
	CreatorName                     *string            `json:"creator_name,omitempty"`
	CreatorID                       *string            `json:"creator_id,omitempty"`
	CreatedAt                       *string            `json:"created_at,omitempty"`           // RFC3339 string
	LastModifiedAt                  *string            `json:"last_modified_at,omitempty"`     // RFC3339 string
	LastModifiedBy                  *string            `json:"last_modified_by,omitempty"`
	SurveyTitle                     *string            `json:"survey_title,omitempty"`
	SurveyStatus                    string             `json:"survey_status"` // scheduled, sending, sent, cancelled
	ResponseStatus                  *string            `json:"response_status,omitempty"` // scheduled, open, closed
	SurveySendDate                  *string            `json:"survey_send_date,omitempty"`     // RFC3339 string
	SurveyCutoffDate                *string            `json:"survey_cutoff_date,omitempty"`   // RFC3339 string
	SurveyReminderRateDays          *int               `json:"survey_reminder_rate_days,omitempty"`
	EmailSubject                    *string            `json:"email_subject,omitempty"`
	EmailBody                       *string            `json:"email_body,omitempty"`           // HTML
	EmailBodyText                   *string            `json:"email_body_text,omitempty"`      // Plain text
	CommitteeCategory               *string            `json:"committee_category,omitempty"`
	Committees                      []SurveyCommittee  `json:"committees,omitempty"`
	CommitteeVotingEnabled          *bool              `json:"committee_voting_enabled,omitempty"`
	SurveyURL                       *string            `json:"survey_url,omitempty"`
	SendImmediately                 *bool              `json:"send_immediately,omitempty"`
	TotalRecipients                 *int               `json:"total_recipients,omitempty"`
	TotalResponses                  *int               `json:"total_responses,omitempty"`
	IsNPSSurvey                     *bool              `json:"is_nps_survey,omitempty"`
	NPSValue                        *float64           `json:"nps_value,omitempty"`
	NumPromoters                    *int               `json:"num_promoters,omitempty"`
	NumPassives                     *int               `json:"num_passives,omitempty"`
	NumDetractors                   *int               `json:"num_detractors,omitempty"`
	TotalBouncedEmails              *int               `json:"total_bounced_emails,omitempty"`
	NumAutomatedRemindersToSend     *int               `json:"num_automated_reminders_to_send,omitempty"`
	NumAutomatedRemindersSent       *int               `json:"num_automated_reminders_sent,omitempty"`
	NextAutomatedReminderAt         *string            `json:"next_automated_reminder_at,omitempty"`         // RFC3339 string
	LatestAutomatedReminderSentAt   *string            `json:"latest_automated_reminder_sent_at,omitempty"`  // RFC3339 string
}

// SurveyCommittee represents a committee associated with a survey
type SurveyCommittee struct {
	CommitteeName   *string  `json:"committee_name,omitempty"`
	CommitteeID     *string  `json:"committee_id,omitempty"`
	ProjectID       *string  `json:"project_id,omitempty"`
	ProjectName     *string  `json:"project_name,omitempty"`
	SurveyURL       *string  `json:"survey_url,omitempty"`
	TotalRecipients *int     `json:"total_recipients,omitempty"`
	TotalResponses  *int     `json:"total_responses,omitempty"`
	NPSValue        *float64 `json:"nps_value,omitempty"`
}

// UpdateSurveyRequest represents the request to update a survey
type UpdateSurveyRequest struct {
	CreatorID              *string  `json:"creator_id,omitempty"`
	SurveyTitle            *string  `json:"survey_title,omitempty"`
	SurveySendDate         *string  `json:"survey_send_date,omitempty"`      // RFC3339 string
	SurveyCutoffDate       *string  `json:"survey_cutoff_date,omitempty"`    // RFC3339 string
	SurveyReminderRateDays *int     `json:"survey_reminder_rate_days,omitempty"`
	EmailSubject           *string  `json:"email_subject,omitempty"`
	EmailBody              *string  `json:"email_body,omitempty"`
	EmailBodyText          *string  `json:"email_body_text,omitempty"`
	Committees             []string `json:"committees,omitempty"`
	CommitteeVotingEnabled *bool    `json:"committee_voting_enabled,omitempty"`
}

// ExtendSurveyRequest represents the request to extend a survey's cutoff date
type ExtendSurveyRequest struct {
	SurveyCutoffDate string `json:"survey_cutoff_date"` // RFC3339 string
}

// BulkResendRequest represents the request to bulk resend survey emails
type BulkResendRequest struct {
	RecipientIDs []string `json:"recipient_ids"`
}

// PreviewSendResponse represents the response from preview_send endpoint
type PreviewSendResponse struct {
	AffectedProjects    []LFXProject              `json:"affected_projects,omitempty"`
	AffectedCommittees  []ExcludedCommittee       `json:"affected_committees,omitempty"`
	AffectedRecipients  []ITXPreviewRecipient     `json:"affected_recipients,omitempty"`
}

// LFXProject represents a project in the preview send response
type LFXProject struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Slug   string  `json:"slug"`
	Status string  `json:"status"`
	LogoURL *string `json:"logo_url,omitempty"`
}

// ExcludedCommittee represents a committee in the preview send response
type ExcludedCommittee struct {
	ProjectID         string  `json:"project_id"`
	ProjectName       string  `json:"project_name"`
	CommitteeID       string  `json:"committee_id"`
	CommitteeName     string  `json:"committee_name"`
	CommitteeCategory string  `json:"committee_category"`
}

// ITXPreviewRecipient represents a recipient in the preview send response
type ITXPreviewRecipient struct {
	UserID    string  `json:"user_id"`
	Name      *string `json:"name,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Email     string  `json:"email"`
	Role      *string `json:"role,omitempty"`
}

// SurveyResults represents aggregated survey results
type SurveyResults struct {
	SurveyResults    []SurveyResultItem `json:"survey_results"`
	CommentResults   []CommentResult    `json:"comment_results,omitempty"`
	NumRecipients    int                `json:"num_recipients"`
	NumResponses     int                `json:"num_responses"`
	SurveyEndTime    *time.Time         `json:"survey_end_time,omitempty"`
}

// SurveyResultItem represents results for a single survey question
type SurveyResultItem struct {
	QuestionID   string                 `json:"question_id"`
	QuestionText string                 `json:"question_text"`
	QuestionType string                 `json:"question_type"`
	Responses    []QuestionResponse     `json:"responses"`
}

// QuestionResponse represents a response summary for a question
type QuestionResponse struct {
	Answer       string  `json:"answer"`
	Count        int     `json:"count"`
	Percentage   float64 `json:"percentage"`
}

// CommentResult represents comment/text responses
type CommentResult struct {
	QuestionID   string   `json:"question_id"`
	QuestionText string   `json:"question_text"`
	Comments     []string `json:"comments"`
}

// SurveyResponse represents a response from a survey participant
type SurveyResponse struct {
	SurveyResponseUID string              `json:"survey_response_uid"`
	SurveyUID         string              `json:"survey_uid"`
	ProjectUID        string              `json:"project_uid"`
	ResponseStatus    string              `json:"response_status"` // submitted, in_progress
	SubmittedAt       *time.Time          `json:"submitted_at,omitempty"`
	UserName          *string             `json:"user_name,omitempty"`
	UserEmail         *string             `json:"user_email,omitempty"`
	Answers           []SurveyAnswer      `json:"answers,omitempty"`
}

// SurveyAnswer represents an answer to a survey question
type SurveyAnswer struct {
	QuestionID   string   `json:"question_id"`
	AnswerText   *string  `json:"answer_text,omitempty"`   // For text questions
	ChoiceIDs    []string `json:"choice_ids,omitempty"`    // For multiple_choice
	RatingValue  *int     `json:"rating_value,omitempty"`  // For rating questions
	YesNoValue   *bool    `json:"yes_no_value,omitempty"`  // For yes_no questions
}

// CreateSurveyResponseRequest represents the request to submit a survey response
type CreateSurveyResponseRequest struct {
	SurveyResponseUID string         `json:"survey_response_uid"`
	SurveyUID         string         `json:"survey_uid"`
	Answers           []SurveyAnswer `json:"answers"`
}

// UpdateSurveyResponseRequest represents the request to update a survey response
type UpdateSurveyResponseRequest struct {
	Answers []SurveyAnswer `json:"answers"`
}

// CreateResponseRequest is an alias for CreateSurveyResponseRequest
type CreateResponseRequest = CreateSurveyResponseRequest

// UpdateResponseRequest is an alias for UpdateSurveyResponseRequest
type UpdateResponseRequest = UpdateSurveyResponseRequest

// ResponseResponse is an alias for SurveyResponse (participant response)
type ResponseResponse = SurveyResponse

// ExclusionRequest represents the request to create or delete an exclusion
type ExclusionRequest struct {
	Email           *string `json:"email,omitempty"`
	UserID          *string `json:"user_id,omitempty"`
	SurveyID        *string `json:"survey_id,omitempty"`
	CommitteeID     *string `json:"committee_id,omitempty"`
	GlobalExclusion *string `json:"global_exclusion,omitempty"`
}

// Exclusion represents a survey or global exclusion
type Exclusion struct {
	ID              string  `json:"id"`
	Email           *string `json:"email,omitempty"`
	SurveyID        *string `json:"survey_id,omitempty"`
	CommitteeID     *string `json:"committee_id,omitempty"`
	GlobalExclusion *string `json:"global_exclusion,omitempty"`
	UserID          *string `json:"user_id,omitempty"`
}

// UserEmail represents an email address for a user
type UserEmail struct {
	ID           *string `json:"id,omitempty"`
	EmailAddress *string `json:"email_address,omitempty"`
	IsPrimary    *bool   `json:"is_primary,omitempty"`
}

// ExclusionUser represents the user information in an extended exclusion
type ExclusionUser struct {
	ID       *string      `json:"id,omitempty"`
	Username *string      `json:"username,omitempty"`
	Emails   []UserEmail  `json:"emails,omitempty"`
}

// ExtendedExclusion represents an exclusion with user information
type ExtendedExclusion struct {
	Exclusion
	User *ExclusionUser `json:"user,omitempty"`
}

// ValidateEmailRequest represents the request to validate email templates
type ValidateEmailRequest struct {
	Body    *string `json:"body,omitempty"`
	Subject *string `json:"subject,omitempty"`
}

// ValidateEmailResponse represents the response from email validation
type ValidateEmailResponse struct {
	Body    string `json:"body"`
	Subject string `json:"subject"`
}
