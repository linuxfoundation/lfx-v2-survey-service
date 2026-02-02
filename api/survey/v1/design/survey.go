// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // Goa DSL convention requires dot imports
)

// JWTAuth defines JWT security for the API
var JWTAuth = JWTSecurity("jwt", func() {
	Description("Heimdall JWT authorization")
	Scope("read:projects", "Read project data")
	Scope("manage:projects", "Manage projects")
	Scope("manage:surveys", "Manage surveys")
})

var _ = API("lfx-v2-survey-service", func() {
	Title("LFX V2 - Survey Service")
	Description("Proxy service for ITX survey system")
	Version("1.0")

	Server("survey-api", func() {
		Host("localhost", func() {
			URI("http://localhost:8080")
		})
	})
})

var _ = Service("survey", func() {
	Description("Survey service that proxies to ITX survey API")

	Security(JWTAuth)

	// Common error responses
	Error("BadRequest", BadRequestError, "Bad request")
	Error("Unauthorized", UnauthorizedError, "Unauthorized")
	Error("Forbidden", ForbiddenError, "Forbidden")
	Error("NotFound", NotFoundError, "Not found")
	Error("Conflict", ConflictError, "Conflict")
	Error("InternalServerError", InternalServerError, "Internal server error")
	Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

	Method("schedule_survey", func() {
		Description("Create a scheduled survey for ITX project committees (proxies to ITX POST /surveys/schedule)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("is_project_survey", Boolean, "Whether the survey is project-level (true) or global-level (false)")
			Attribute("stage_filter", String, "Project stage filter for global surveys")
			Attribute("creator_username", String, "Creator's username")
			Attribute("creator_name", String, "Creator's full name")
			Attribute("creator_id", String, "Creator's user ID")
			Attribute("survey_monkey_id", String, "SurveyMonkey survey ID")
			Attribute("survey_title", String, "Survey title")
			Attribute("send_immediately", Boolean, "Send immediately (true) or schedule for later (false)")
			Attribute("survey_send_date", String, "Date to send the survey (RFC3339 format)")
			Attribute("survey_cutoff_date", String, "Survey cutoff/end date (RFC3339 format)")
			Attribute("survey_reminder_rate_days", Int, "Days between automatic reminder emails (0 = no reminders)")
			Attribute("email_subject", String, "Email subject line")
			Attribute("email_body", String, "Email body HTML content")
			Attribute("email_body_text", String, "Email body plain text content")
			Attribute("committees", ArrayOf(String), "Array of committee IDs to send survey to")
			Attribute("committee_voting_enabled", Boolean, "Whether committee voting is enabled")

			// No required fields per the ITX API spec
		})

		Result(SurveyScheduleResult)

		HTTP(func() {
			POST("/surveys/schedule")

			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})
})
