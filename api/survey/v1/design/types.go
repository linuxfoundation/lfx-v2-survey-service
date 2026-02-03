// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // Goa DSL convention requires dot imports
)

//
// Reusable Attribute Functions
//

// BearerTokenAttribute is a reusable token attribute for JWT authentication.
func BearerTokenAttribute() {
	Token("token", String, "JWT token", func() {
		Example("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...")
	})
}

//
// Type Definitions
//

// SurveySchedulePayload represents the payload for scheduling a survey
var SurveySchedulePayload = Type("SurveySchedulePayload", func() {
	Description("Payload for scheduling a survey")

	Attribute("is_project_survey", Boolean, "Whether the survey is project-level (true) or global-level (false)", func() {
		Default(false)
		Example(false)
	})

	Attribute("stage_filter", String, "Project stage filter for global surveys", func() {
		Example("Formation - Engaged")
	})

	Attribute("creator_username", String, "Creator's username", func() {
		Example("john23")
	})

	Attribute("creator_name", String, "Creator's full name", func() {
		Example("John Doe")
	})

	Attribute("creator_id", String, "Creator's user ID", func() {
		Example("john-doe-id")
	})

	Attribute("survey_monkey_id", String, "SurveyMonkey survey ID", func() {
		Example("123456789")
	})

	Attribute("survey_title", String, "Survey title", func() {
		Example("Q1 2026 Developer Satisfaction Survey")
		MaxLength(255)
	})

	Attribute("send_immediately", Boolean, "Send immediately (true) or schedule for later (false)", func() {
		Default(false)
		Example(false)
	})

	Attribute("survey_send_date", String, "Date to send the survey (RFC3339 format)", func() {
		Format(FormatDateTime)
		Example("2026-02-15T09:00:00Z")
	})

	Attribute("survey_cutoff_date", String, "Survey cutoff/end date (RFC3339 format)", func() {
		Format(FormatDateTime)
		Example("2026-03-15T09:00:00Z")
	})

	Attribute("survey_reminder_rate_days", Int, "Days between automatic reminder emails (0 = no reminders)", func() {
		Example(7)
		Minimum(0)
	})

	Attribute("email_subject", String, "Email subject line", func() {
		Example("You're invited: Q1 2026 Developer Survey")
		MaxLength(200)
	})

	Attribute("email_body", String, "Email body HTML content", func() {
		Example("<!DOCTYPE html><html><body><h3>Hi there</h3><p>Please take our survey</p></body></html>")
	})

	Attribute("email_body_text", String, "Email body plain text content", func() {
		Example("Hi there! Please take our survey at: https://surveymonkey.com/...")
	})

	Attribute("committees", ArrayOf(String), "Array of committee IDs to send survey to", func() {
		Example([]string{"com-123", "com-456"})
	})

	Attribute("committee_voting_enabled", Boolean, "Whether committee voting is enabled", func() {
		Default(false)
		Example(true)
	})

	// No required fields per the ITX API spec
})

// SurveyScheduleResult represents a scheduled survey response
var SurveyScheduleResult = Type("SurveyScheduleResult", func() {
	Description("Scheduled survey details")

	Attribute("id", String, "Survey identifier", func() {
		Example("4e8165a9-9b29-4506-b093-ab0a4aae9b84")
	})

	Attribute("survey_monkey_id", String, "SurveyMonkey survey ID")

	Attribute("is_project_survey", Boolean, "Whether project-level or global-level survey")

	Attribute("stage_filter", String, "Project stage filter")

	Attribute("creator_username", String, "Creator's username")
	Attribute("creator_name", String, "Creator's full name")
	Attribute("creator_id", String, "Creator's user ID")

	Attribute("created_at", String, "Creation timestamp", func() {
		Format(FormatDateTime)
	})

	Attribute("last_modified_at", String, "Last modification timestamp", func() {
		Format(FormatDateTime)
	})

	Attribute("last_modified_by", String, "User ID of last modifier")

	Attribute("survey_title", String, "Survey title")

	Attribute("survey_status", String, "Survey status", func() {
		Enum("scheduled", "sending", "sent", "cancelled")
		Example("scheduled")
	})

	Attribute("response_status", String, "Response status", func() {
		Enum("scheduled", "open", "closed")
		Example("scheduled")
	})

	Attribute("survey_send_date", String, "Survey send date", func() {
		Format(FormatDateTime)
	})

	Attribute("survey_cutoff_date", String, "Survey cutoff date", func() {
		Format(FormatDateTime)
	})

	Attribute("survey_reminder_rate_days", Int, "Days between reminder emails")

	Attribute("email_subject", String, "Email subject line")
	Attribute("email_body", String, "Email body HTML")
	Attribute("email_body_text", String, "Email body plain text")

	Attribute("committee_category", String, "Committee category")
	Attribute("committees", ArrayOf(SurveyCommittee), "Survey committees")
	Attribute("committee_voting_enabled", Boolean, "Committee voting enabled")

	Attribute("survey_url", String, "Survey URL")
	Attribute("send_immediately", Boolean, "Whether survey is sent immediately")

	Attribute("total_recipients", Int, "Total number of recipients")
	Attribute("total_responses", Int, "Total number of responses")

	Attribute("is_nps_survey", Boolean, "Whether this is an NPS survey")
	Attribute("nps_value", Float64, "NPS value")
	Attribute("num_promoters", Int, "Number of promoters")
	Attribute("num_passives", Int, "Number of passives")
	Attribute("num_detractors", Int, "Number of detractors")

	Attribute("total_bounced_emails", Int, "Number of bounced emails")
	Attribute("num_automated_reminders_to_send", Int, "Number of automated reminders to send")
	Attribute("num_automated_reminders_sent", Int, "Number of automated reminders sent")

	Attribute("next_automated_reminder_at", String, "Next automated reminder date", func() {
		Format(FormatDateTime)
	})

	Attribute("latest_automated_reminder_sent_at", String, "Latest automated reminder sent date", func() {
		Format(FormatDateTime)
	})

	Required("id", "survey_status")
})

// SurveyCommittee represents a committee associated with a survey
var SurveyCommittee = Type("SurveyCommittee", func() {
	Description("Survey committee details")

	Attribute("committee_name", String, "Committee name", func() {
		Example("Technical Steering Committee")
	})

	Attribute("committee_id", String, "Committee ID", func() {
		Example("qa1e8536-a985-4cf5-b981-a170927a1d11")
	})

	Attribute("project_id", String, "Project ID", func() {
		Example("qa1e8536-a985-4cf5-b981-a170927a1d11")
	})

	Attribute("project_name", String, "Project name", func() {
		Example("Kubernetes")
	})

	Attribute("survey_url", String, "Survey URL for this committee", func() {
		Example("https://surveymonkey.com/r/abc123")
	})

	Attribute("total_recipients", Int, "Total recipients for this committee")
	Attribute("total_responses", Int, "Total responses for this committee")
	Attribute("nps_value", Float64, "NPS value for this committee")
})

// BadRequestError represents a 400 Bad Request error
var BadRequestError = Type("BadRequestError", func() {
	Description("Bad request error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// NotFoundError represents a 404 Not Found error
var NotFoundError = Type("NotFoundError", func() {
	Description("Not found error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// ConflictError represents a 409 Conflict error
var ConflictError = Type("ConflictError", func() {
	Description("Conflict error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// InternalServerError represents a 500 Internal Server Error
var InternalServerError = Type("InternalServerError", func() {
	Description("Internal server error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// ServiceUnavailableError represents a 503 Service Unavailable error
var ServiceUnavailableError = Type("ServiceUnavailableError", func() {
	Description("Service unavailable error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// UnauthorizedError represents a 401 Unauthorized error
var UnauthorizedError = Type("UnauthorizedError", func() {
	Description("Unauthorized error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// ForbiddenError represents a 403 Forbidden error
var ForbiddenError = Type("ForbiddenError", func() {
	Description("Forbidden error response")
	Attribute("code", String, "HTTP status code")
	Attribute("message", String, "Error message")
	Required("code", "message")
})

// PreviewSendResult represents the preview send response
var PreviewSendResult = Type("PreviewSendResult", func() {
	Description("Preview of recipients, committees, and projects affected by a resend")

	Attribute("affected_projects", ArrayOf(LFXProject), "List of affected projects")
	Attribute("affected_committees", ArrayOf(ExcludedCommittee), "List of affected committees")
	Attribute("affected_recipients", ArrayOf(ITXPreviewRecipient), "List of affected recipients")
})

// LFXProject represents a project in the preview send response
var LFXProject = Type("LFXProject", func() {
	Description("LFX Project information")

	Attribute("id", String, "Project ID", func() {
		Example("003170000123XHTAA2")
	})

	Attribute("name", String, "Project name", func() {
		Example("Express JS")
	})

	Attribute("slug", String, "Project slug", func() {
		Example("express-gateway")
	})

	Attribute("status", String, "Project status/stage", func() {
		Enum("Formation - Exploratory", "Formation - Engaged", "Active", "Archived", "Formation - On Hold", "Formation - Disengaged", "Formation - Confidential", "Prospect")
		Example("Active")
	})

	Attribute("logo_url", String, "Project logo URL")

	Required("id", "name", "slug", "status")
})

// ExcludedCommittee represents a committee in the preview send response
var ExcludedCommittee = Type("ExcludedCommittee", func() {
	Description("Committee information for preview send")

	Attribute("project_id", String, "Project ID", func() {
		Example("003170000123XHTAA2")
	})

	Attribute("project_name", String, "Project name", func() {
		Example("Kubernetes")
	})

	Attribute("committee_id", String, "Committee ID", func() {
		Example("qa1e8536-a985-4cf5-b981-a170927a1d11")
	})

	Attribute("committee_name", String, "Committee name", func() {
		Example("Technical Steering Committee")
	})

	Attribute("committee_category", String, "Committee category", func() {
		Enum("Legal Committee", "Finance Committee", "Special Interest Group", "Board", "Technical Oversight Committee/Technical Advisory Committee", "Technical Steering Committee")
		Example("Technical Steering Committee")
	})

	Required("project_id", "project_name", "committee_id", "committee_name", "committee_category")
})

// ITXPreviewRecipient represents a recipient in the preview send response
var ITXPreviewRecipient = Type("ITXPreviewRecipient", func() {
	Description("Recipient information for preview send")

	Attribute("user_id", String, "LF user ID", func() {
		Example("005f1000009RbC4AAK")
	})

	Attribute("name", String, "User full name", func() {
		Example("John Doe")
	})

	Attribute("first_name", String, "User first name", func() {
		Example("John")
	})

	Attribute("last_name", String, "User last name", func() {
		Example("Doe")
	})

	Attribute("username", String, "Linux Foundation ID", func() {
		Example("jdoe")
	})

	Attribute("email", String, "Email address", func() {
		Format(FormatEmail)
		Example("john.doe@example.com")
	})

	Attribute("role", String, "Role in committee", func() {
		Enum("Chair", "Voting Rep", "Member")
		Example("Voting Rep")
	})

	Required("user_id", "email")
})
