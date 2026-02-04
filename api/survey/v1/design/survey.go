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
			POST("/surveys")

			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get_survey", func() {
		Description("Get survey details (proxies to ITX GET /v2/surveys/{survey_id})")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Required("survey_id")
		})

		Result(SurveyScheduleResult)

		HTTP(func() {
			GET("/surveys/{survey_id}")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("update_survey", func() {
		Description("Update survey (proxies to ITX PUT /v2/surveys/{survey_id}). Only allowed when status is 'disabled'")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})
			Attribute("creator_id", String, "Creator's user ID")
			Attribute("survey_title", String, "Survey title")
			Attribute("survey_send_date", String, "Date to send the survey (RFC3339 format)")
			Attribute("survey_cutoff_date", String, "Survey cutoff/end date (RFC3339 format)")
			Attribute("survey_reminder_rate_days", Int, "Days between automatic reminder emails (0 = no reminders)")
			Attribute("email_subject", String, "Email subject line")
			Attribute("email_body", String, "Email body HTML content")
			Attribute("email_body_text", String, "Email body plain text content")
			Attribute("committees", ArrayOf(String), "Array of committee IDs to send survey to")
			Attribute("committee_voting_enabled", Boolean, "Whether committee voting is enabled")

			Required("survey_id")
		})

		Result(SurveyScheduleResult)

		HTTP(func() {
			PUT("/surveys/{survey_id}")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete_survey", func() {
		Description("Delete survey (proxies to ITX DELETE /v2/surveys/{survey_id}). Only allowed when status is 'disabled'")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Required("survey_id")
		})

		HTTP(func() {
			DELETE("/surveys/{survey_id}")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("bulk_resend_survey", func() {
		Description("Bulk resend survey emails to select recipients (proxies to ITX POST /v2/surveys/{survey_id}/bulk_resend)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Attribute("recipient_ids", ArrayOf(String), "Array of recipient IDs to resend survey emails to", func() {
				Example([]string{"cba14f40-1636-11ec-9621-0242ac130002", "cba14f40-1636-11ec-9621-0242ac130003"})
			})

			Required("survey_id", "recipient_ids")
		})

		HTTP(func() {
			POST("/surveys/{survey_id}/bulk_resend")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("preview_send_survey", func() {
		Description("Preview which recipients, committees, and projects would be affected by a resend (proxies to ITX GET /v2/surveys/{survey_id}/preview_send)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Attribute("committee_id", String, "Optional committee ID to filter preview", func() {
				Example("qa1e8536-a985-4cf5-b981-a170927a1d11")
			})

			Required("survey_id")
		})

		Result(PreviewSendResult)

		HTTP(func() {
			GET("/surveys/{survey_id}/preview_send")
			Param("committee_id")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("send_missing_recipients", func() {
		Description("Send survey emails to committee members who haven't received it (proxies to ITX POST /v2/surveys/{survey_id}/send_missing_recipients)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Attribute("committee_id", String, "Optional committee ID to resync only that committee", func() {
				Example("qa1e8536-a985-4cf5-b981-a170927a1d11")
			})

			Required("survey_id")
		})

		HTTP(func() {
			POST("/surveys/{survey_id}/send_missing_recipients")
			Param("committee_id")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete_survey_response", func() {
		Description("Delete survey response - removes recipient from survey and recalculates statistics (proxies to ITX DELETE /v2/surveys/{survey_id}/responses/{response_id})")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Attribute("response_id", String, "Response identifier", func() {
				Example("cba14f40-1636-11ec-9621-0242ac130002")
			})

			Required("survey_id", "response_id")
		})

		HTTP(func() {
			DELETE("/surveys/{survey_id}/responses/{response_id}")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("resend_survey_response", func() {
		Description("Resend survey email to a specific user (proxies to ITX POST /v2/surveys/{survey_id}/responses/{response_id}/resend)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Attribute("response_id", String, "Response identifier", func() {
				Example("cba14f40-1636-11ec-9621-0242ac130002")
			})

			Required("survey_id", "response_id")
		})

		HTTP(func() {
			POST("/surveys/{survey_id}/responses/{response_id}/resend")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete_recipient_group", func() {
		Description("Remove a recipient group (committee, project, or foundation) from survey and recalculate statistics (proxies to ITX DELETE /v2/surveys/{survey_id}/recipient_group)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("survey_id", String, "Survey identifier", func() {
				Example("b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
			})

			Attribute("committee_id", String, "Committee ID to remove (indicates specific committee in project)", func() {
				Example("qa1e8536-a985-4cf5-b981-a170927a1d11")
			})

			Attribute("project_id", String, "Project ID to remove (all removals are attached to a project)", func() {
				Example("003170000123XHTAA2")
			})

			Attribute("foundation_id", String, "Foundation ID (indicates project_id references a foundation and all subprojects should be removed)", func() {
				Example("003170000123XHTAA2")
			})

			Required("survey_id")
		})

		HTTP(func() {
			DELETE("/surveys/{survey_id}/recipient_group")
			Param("committee_id")
			Param("project_id")
			Param("foundation_id")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("create_exclusion", func() {
		Description("Create a survey or global exclusion (proxies to ITX POST /v2/surveys/exclusion)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("email", String, "Survey responder's email")
			Attribute("user_id", String, "Recipient's user ID")
			Attribute("survey_id", String, "Survey ID for survey-specific exclusion")
			Attribute("committee_id", String, "Committee ID for survey-specific exclusion")
			Attribute("global_exclusion", String, "Global exclusion flag")
		})

		Result(ExclusionResult)

		HTTP(func() {
			POST("/surveys/exclusion")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("InternalServerError", StatusInternalServerError)
		})
	})

	Method("delete_exclusion", func() {
		Description("Delete a survey or global exclusion (proxies to ITX DELETE /v2/surveys/exclusion)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("email", String, "Survey responder's email")
			Attribute("user_id", String, "Recipient's user ID")
			Attribute("survey_id", String, "Survey ID for survey-specific exclusion")
			Attribute("committee_id", String, "Committee ID for survey-specific exclusion")
			Attribute("global_exclusion", String, "Global exclusion flag")
		})

		HTTP(func() {
			DELETE("/surveys/exclusion")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("InternalServerError", StatusInternalServerError)
		})
	})

	Method("get_exclusion", func() {
		Description("Get exclusion by ID (proxies to ITX GET /v2/surveys/exclusion/{exclusion_id})")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("exclusion_id", String, "Exclusion identifier", func() {
				Example("12345")
			})

			Required("exclusion_id")
		})

		Result(ExtendedExclusionResult)

		HTTP(func() {
			GET("/surveys/exclusion/{exclusion_id}")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
		})
	})

	Method("delete_exclusion_by_id", func() {
		Description("Delete exclusion by ID (proxies to ITX DELETE /v2/surveys/exclusion/{exclusion_id})")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("exclusion_id", String, "Exclusion identifier", func() {
				Example("12345")
			})

			Required("exclusion_id")
		})

		HTTP(func() {
			DELETE("/surveys/exclusion/{exclusion_id}")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("InternalServerError", StatusInternalServerError)
		})
	})

	Method("validate_email", func() {
		Description("Validate email template body and subject (proxies to ITX POST /v2/surveys/validate_email)")

		Security(JWTAuth, func() {
			Scope("manage:projects")
			Scope("manage:surveys")
		})

		Payload(func() {
			BearerTokenAttribute()

			Attribute("body", String, "Email body template")
			Attribute("subject", String, "Email subject template")
		})

		Result(ValidateEmailResult)

		HTTP(func() {
			POST("/surveys/validate_email")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("InternalServerError", StatusInternalServerError)
		})
	})
})
