// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

// SurveyData represents v2 survey data after transformation from v1 format
type SurveyData struct {
	UID                    string                `json:"uid"` // v2 UID (same as ID)
	ID                     string                `json:"id"`  // v1 ID
	SurveyMonkeyID         string                `json:"survey_monkey_id"`
	IsProjectSurvey        bool                  `json:"is_project_survey"`
	StageFilter            string                `json:"stage_filter"`
	CreatorUsername        string                `json:"creator_username"`
	CreatorName            string                `json:"creator_name"`
	CreatorID              string                `json:"creator_id"`
	CreatedAt              string                `json:"created_at"`
	LastModifiedAt         string                `json:"last_modified_at"`
	LastModifiedBy         string                `json:"last_modified_by"`
	SurveyTitle            string                `json:"survey_title"`
	SurveySendDate         string                `json:"survey_send_date"`
	SurveyCutoffDate       string                `json:"survey_cutoff_date"`
	SurveyReminderRateDays int                   `json:"survey_reminder_rate_days"`
	SendImmediately        bool                  `json:"send_immediately"`
	EmailSubject           string                `json:"email_subject"`
	EmailBody              string                `json:"email_body"`
	EmailBodyText          string                `json:"email_body_text"`
	CommitteeCategory      string                `json:"committee_category"`
	Committees             []SurveyCommitteeData `json:"committees"`
	CommitteeVotingEnabled bool                  `json:"committee_voting_enabled"`
	SurveyStatus           string                `json:"survey_status"`
	NPSValue               int                   `json:"nps_value"`
	NumPromoters           int                   `json:"num_promoters"`
	NumPassives            int                   `json:"num_passives"`
	NumDetractors          int                   `json:"num_detractors"`
	TotalRecipients        int                   `json:"total_recipients"`
	TotalSentRecipients    int                   `json:"total_recipients_sent"`
	TotalResponses         int                   `json:"total_responses"`
	TotalRecipientsOpened  int                   `json:"total_recipients_opened"`
	TotalRecipientsClicked int                   `json:"total_recipients_clicked"`
	TotalDeliveryErrors    int                   `json:"total_delivery_errors"`
	IsNPSSurvey            bool                  `json:"is_nps_survey"`
	CollectorURL           string                `json:"collector_url"`
}

// SurveyCommitteeData represents committee data with v2 UIDs
type SurveyCommitteeData struct {
	CommitteeUID           string `json:"committee_uid"` // v2 UID
	CommitteeID            string `json:"committee_id"`  // v1 SFID
	CommitteeName          string `json:"committee_name"`
	ProjectUID             string `json:"project_uid"` // v2 UID
	ProjectID              string `json:"project_id"`  // v1 SFID
	ProjectName            string `json:"project_name"`
	NPSValue               int    `json:"nps_value"`
	NumPromoters           int    `json:"num_promoters"`
	NumPassives            int    `json:"num_passives"`
	NumDetractors          int    `json:"num_detractors"`
	TotalRecipients        int    `json:"total_recipients"`
	TotalSentRecipients    int    `json:"total_recipients_sent"`
	TotalResponses         int    `json:"total_responses"`
	TotalRecipientsOpened  int    `json:"total_recipients_opened"`
	TotalRecipientsClicked int    `json:"total_recipients_clicked"`
	TotalDeliveryErrors    int    `json:"total_delivery_errors"`
}

// SurveyResponseData represents v2 survey response data after transformation from v1 format
type SurveyResponseData struct {
	UID                           string                        `json:"uid"`        // v2 UID (same as ID)
	ID                            string                        `json:"id"`         // v1 ID
	SurveyID                      string                        `json:"survey_id"`  // v1 survey ID
	SurveyUID                     string                        `json:"survey_uid"` // v2 survey UID
	SurveyMonkeyRespondent        string                        `json:"survey_monkey_respondent_id"`
	Email                         string                        `json:"email"`
	CommitteeMemberID             string                        `json:"committee_member_id,omitempty"`
	FirstName                     string                        `json:"first_name"`
	LastName                      string                        `json:"last_name"`
	CreatedAt                     string                        `json:"created_at"`
	ResponseDatetime              string                        `json:"response_datetime"`
	LastReceivedTime              string                        `json:"last_received_time"`
	NumAutomatedRemindersReceived int                           `json:"num_automated_reminders_received"`
	Username                      string                        `json:"username"`
	VotingStatus                  string                        `json:"voting_status"`
	Role                          string                        `json:"role"`
	JobTitle                      string                        `json:"job_title"`
	MembershipTier                string                        `json:"membership_tier"`
	Organization                  SurveyResponseOrgData         `json:"organization"`
	Project                       SurveyResponseProjectData     `json:"project"`
	CommitteeUID                  string                        `json:"committee_uid"` // v2 UID
	CommitteeID                   string                        `json:"committee_id"`  // v1 SFID
	CommitteeVotingEnabled        bool                          `json:"committee_voting_enabled"`
	SurveyLink                    string                        `json:"survey_link"`
	NPSValue                      int                           `json:"nps_value"`
	SurveyMonkeyQuestionAnswers   []SurveyMonkeyQuestionAnswers `json:"survey_monkey_question_answers"`
	SESMessageID                  string                        `json:"ses_message_id"`
	SESBounceType                 string                        `json:"ses_bounce_type"`
	SESBounceSubtype              string                        `json:"ses_bounce_subtype"`
	SESBounceDiagnosticCode       string                        `json:"ses_bounce_diagnostic_code"`
	SESComplaintExists            bool                          `json:"ses_complaint_exists"`
	SESComplaintType              string                        `json:"ses_complaint_type"`
	SESComplaintDate              string                        `json:"ses_complaint_date"`
	SESDeliverySuccessful         bool                          `json:"ses_delivery_successful"`
	EmailOpenedFirstTime          string                        `json:"email_opened_first_time"`
	EmailOpenedLastTime           string                        `json:"email_opened_last_time"`
	LinkClickedFirstTime          string                        `json:"link_clicked_first_time"`
	LinkClickedLastTime           string                        `json:"link_clicked_last_time"`
	Excluded                      bool                          `json:"excluded"`
}

// SurveyMonkeyQuestionAnswers contains a SurveyMonkey response
type SurveyMonkeyQuestionAnswers struct {
	QuestionID      string               `json:"question_id"`
	QuestionText    string               `json:"question_text"`
	QuestionFamily  string               `json:"question_family"`
	QuestionSubtype string               `json:"question_subtype"`
	Answers         []SurveyMonkeyAnswer `json:"answers"`
}

// SurveyMonkeyAnswer contains a SurveyMonkey answer to a question
type SurveyMonkeyAnswer struct {
	ChoiceID string `json:"choice_id"`
	Text     string `json:"text"`
}

// SurveyResponseProjectData contains project data with v2 UIDs
type SurveyResponseProjectData struct {
	ProjectUID string `json:"project_uid"` // v2 UID
	ID         string `json:"id"`          // v1 SFID
	Name       string `json:"name"`
}

// SurveyResponseOrgData contains organization data
type SurveyResponseOrgData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
