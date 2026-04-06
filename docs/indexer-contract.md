# Indexer Contract — Survey Service

This document is the authoritative reference for all data the survey service sends to the indexer service, which makes resources searchable via the [query service](https://github.com/linuxfoundation/lfx-v2-query-service).

**Update this document in the same PR as any change to indexer message construction.**

---

## Resource Types

- [Survey](#survey)
- [Survey Response](#survey-response)
- [Survey Template](#survey-template)

---

## Survey

**Source struct:** `internal/domain/event_models.go` — `SurveyData`

**Indexed on:** create, update, delete of a survey (sourced from the `itx-surveys` KV bucket).

### Data Schema

These fields are indexed and queryable via `filters` or `cel_filter` in the query service.

| Field | Type | Description |
|---|---|---|
| `uid` | string | Survey unique identifier (v2 UUID) |
| `id` | string | v1 ID |
| `survey_monkey_id` | string | SurveyMonkey survey ID |
| `is_project_survey` | bool | Whether this is a project-level survey |
| `stage_filter` | string | Stage filter value |
| `creator_username` | string | Username of the survey creator |
| `creator_name` | string | Display name of the survey creator |
| `creator_id` | string | ID of the survey creator |
| `created_at` | string | Creation time (RFC3339) |
| `last_modified_at` | string | Last modification time (RFC3339) |
| `last_modified_by` | string | Username who last modified the survey |
| `survey_title` | string | Survey title |
| `survey_send_date` | string | Scheduled send date (RFC3339) |
| `survey_cutoff_date` | string | Response cutoff date (RFC3339) |
| `survey_reminder_rate_days` | int | Days between automated reminders |
| `send_immediately` | bool | Whether the survey is sent immediately |
| `email_subject` | string | Email subject line |
| `email_body` | string | Email body HTML |
| `email_body_text` | string | Email body plain text |
| `committee_category` | string | Category of committees this survey targets |
| `committees` | []object | Array of associated committees (see [SurveyCommitteeData](#surveycommitteedata)) |
| `committee_voting_enabled` | bool | Whether committee voting is enabled for this survey |
| `survey_status` | string | Current survey status |
| `nps_value` | int | Net Promoter Score value |
| `num_promoters` | int | Number of promoter responses |
| `num_passives` | int | Number of passive responses |
| `num_detractors` | int | Number of detractor responses |
| `total_recipients` | int | Total intended recipients |
| `total_recipients_sent` | int | Total recipients the survey was sent to |
| `total_responses` | int | Total responses received |
| `total_recipients_opened` | int | Total recipients who opened the email |
| `total_recipients_clicked` | int | Total recipients who clicked the survey link |
| `total_delivery_errors` | int | Total delivery error count |
| `is_nps_survey` | bool | Whether this is an NPS survey |
| `collector_url` | string | SurveyMonkey collector URL |

#### SurveyCommitteeData

Each entry in the `committees` array has the following fields:

| Field | Type | Description |
|---|---|---|
| `committee_uid` | string | Committee v2 UUID |
| `committee_id` | string | Committee v1 Salesforce ID |
| `committee_name` | string | Committee name |
| `project_uid` | string | Project v2 UUID |
| `project_id` | string | Project v1 Salesforce ID |
| `project_name` | string | Project name |
| `nps_value` | int | NPS value for this committee |
| `num_promoters` | int | Promoter count for this committee |
| `num_passives` | int | Passive count for this committee |
| `num_detractors` | int | Detractor count for this committee |
| `total_recipients` | int | Total recipients in this committee |
| `total_recipients_sent` | int | Sent count for this committee |
| `total_responses` | int | Response count for this committee |
| `total_recipients_opened` | int | Opened count for this committee |
| `total_recipients_clicked` | int | Clicked count for this committee |
| `total_delivery_errors` | int | Delivery errors for this committee |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `committee_uid:{uid}` | `committee_uid:061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Find surveys for a committee |
| `project_uid:{uid}` | `project_uid:cbef1ed5-17dc-4a50-84e2-6cddd70f6878` | Find surveys for a project |

> One `committee_uid` tag is emitted per committee in the `committees` array. One `project_uid` tag is emitted per unique project (deduplicated across all committees). Both tags are only emitted when the respective UID is non-empty.

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `survey:{uid}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `survey:{uid}` |
| `history_check_relation` | `auditor` |

### FGA-Sync Access Message

On create/update, a message is published to `lfx.fga-sync.update_access`:

```json
{
  "object_type": "survey",
  "operation": "update_access",
  "data": {
    "uid": "<survey_uid>",
    "public": false,
    "references": {
      "committee": ["<committee_uid>", ...],
      "project": ["<project_uid>", ...]
    }
  }
}
```

> The access message is only sent when at least one valid committee or project reference exists. On delete, a `lfx.fga-sync.delete_access` message is sent with only `uid`.

### Search Behavior

| Field | Value |
|---|---|
| `fulltext` | `survey_title` |
| `name_and_aliases` | `survey_title` (when non-empty) |
| `sort_name` | `survey_title` |
| `public` | `false` (always) |

### Parent References

| Ref | Condition |
|---|---|
| `committee:{committee_uid}` | For each committee in `committees` with a non-empty `committee_uid` |
| `project:{project_uid}` | For each unique project across all committees (deduplicated) |

---

## Survey Response

**Source struct:** `internal/domain/event_models.go` — `SurveyResponseData`

**Indexed on:** create, update, delete of a survey response (sourced from the `itx-survey-responses` KV bucket).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Response unique identifier (v2 UUID) |
| `id` | string | v1 ID |
| `survey_id` | string | v1 parent survey ID |
| `survey_uid` | string | v2 parent survey UUID |
| `survey_monkey_respondent_id` | string | SurveyMonkey respondent ID |
| `email` | string | Respondent email address |
| `committee_member_id` | string (optional) | Committee member ID |
| `first_name` | string | Respondent first name |
| `last_name` | string | Respondent last name |
| `created_at` | string | Creation time (RFC3339) |
| `response_datetime` | string | Time the response was submitted (RFC3339) |
| `last_received_time` | string | Last time the response was received (RFC3339) |
| `num_automated_reminders_received` | int | Number of automated reminders received |
| `username` | string | Respondent's LFX username |
| `voting_status` | string | Respondent's voting status in the committee |
| `role` | string | Respondent's role in the committee |
| `job_title` | string | Respondent's job title |
| `membership_tier` | string | Respondent's membership tier |
| `organization.id` | string | Respondent's organization ID |
| `organization.name` | string | Respondent's organization name |
| `project.project_uid` | string | Parent project v2 UUID |
| `project.id` | string | Parent project v1 Salesforce ID |
| `project.name` | string | Parent project name |
| `committee_uid` | string | Parent committee v2 UUID |
| `committee_id` | string | Parent committee v1 Salesforce ID |
| `committee_voting_enabled` | bool | Whether committee voting was enabled for this survey |
| `survey_link` | string | Link to the survey |
| `nps_value` | int | NPS score from this response |
| `survey_monkey_question_answers` | []object | Array of question answers (see below) |
| `ses_message_id` | string | AWS SES message ID |
| `ses_bounce_type` | string | SES bounce type |
| `ses_bounce_subtype` | string | SES bounce subtype |
| `ses_bounce_diagnostic_code` | string | SES bounce diagnostic code |
| `ses_complaint_exists` | bool | Whether an SES complaint was filed |
| `ses_complaint_type` | string | SES complaint type |
| `ses_complaint_date` | string | SES complaint date |
| `ses_delivery_successful` | bool | Whether SES delivery succeeded |
| `email_opened_first_time` | string | First time the email was opened (RFC3339) |
| `email_opened_last_time` | string | Last time the email was opened (RFC3339) |
| `link_clicked_first_time` | string | First time the survey link was clicked (RFC3339) |
| `link_clicked_last_time` | string | Last time the survey link was clicked (RFC3339) |
| `excluded` | bool | Whether this response is excluded from results |

#### SurveyMonkeyQuestionAnswers

Each entry in `survey_monkey_question_answers` has:

| Field | Type | Description |
|---|---|---|
| `question_id` | string | SurveyMonkey question ID |
| `question_text` | string | Question text |
| `question_family` | string | Question family (e.g., `single_choice`) |
| `question_subtype` | string | Question subtype |
| `answers` | []object | Array of answers, each with `choice_id` (string) and `text` (string) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `project_uid:{uid}` | `project_uid:cbef1ed5-17dc-4a50-84e2-6cddd70f6878` | Find responses for a project |
| `committee_uid:{uid}` | `committee_uid:061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Find responses for a committee |
| `survey_uid:{uid}` | `survey_uid:a1b2c3d4-e5f6-7890-abcd-ef1234567890` | Find responses for a survey |

> `project_uid` is only emitted when `project.project_uid` is non-empty. `committee_uid` is only emitted when `committee_uid` is non-empty. `survey_uid` is only emitted when `survey_uid` is non-empty.

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `survey:{survey_uid}` (access is checked on the **parent survey**, not the response itself) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `survey_response:{uid}` |
| `history_check_relation` | `auditor` |

### FGA-Sync Access Message

On create/update, a message is published to `lfx.fga-sync.update_access`:

```json
{
  "object_type": "survey_response",
  "operation": "update_access",
  "data": {
    "uid": "<response_uid>",
    "public": false,
    "relations": {
      "writer": ["<username>"],
      "viewer": ["<username>"]
    },
    "references": {
      "project": ["<project_uid>"],
      "survey": ["<survey_uid>"]
    }
  }
}
```

> When an access message is published, the `relations` and `references` keys may be present even if they are empty maps. A non-empty `username` populates `relations.writer` and `relations.viewer`. Non-empty project and survey UIDs populate `references.project` and `references.survey`, respectively. The access message is skipped entirely if both `relations` and `references` are empty. On delete, a `lfx.fga-sync.delete_access` message is sent with only `uid`.

### Search Behavior

| Field | Value |
|---|---|
| `fulltext` | `email first_name last_name` (space-joined) |
| `name_and_aliases` | `email` (when non-empty) |
| `sort_name` | `email` |
| `public` | `false` (always) |

### Parent References

| Ref | Condition |
|---|---|
| `project:{project_uid}` | Only when `project.project_uid` is non-empty |
| `committee:{committee_uid}` | Only when `committee_uid` is non-empty |
| `survey:{survey_uid}` | Only when `survey_uid` is non-empty |

---

## Survey Template

**Source struct:** `internal/domain/event_models.go` — `SurveyTemplateData`

**Indexed on:** create, update, delete of a survey template (sourced from the `surveymonkey-surveys` KV bucket).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | SurveyMonkey survey ID (used as the object ID) |
| `title` | string | Template title |
| `href` | string | SurveyMonkey API href |
| `nickname` | string | Template nickname |
| `question_count` | int | Number of questions in the template |
| `analyze_url` | string | SurveyMonkey analyze URL |
| `edit_url` | string | SurveyMonkey edit URL |
| `collect_url` | string | SurveyMonkey collect URL |
| `preview` | string | Preview URL or content |
| `date_created` | string | Creation date (RFC3339) |
| `date_modified` | string | Last modification date (RFC3339) |
| `language` | string | Survey language code |
| `folder_id` | string | SurveyMonkey folder ID |
| `page_count` | int | Number of pages |
| `category` | string | Template category |
| `is_owner` | bool | Whether the LFX platform owns this template |
| `custom_variables` | map[string]string | Custom variable key-value pairs |

### Tags

_(none)_

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `team:global_survey_platform_admins` |
| `access_check_relation` | `member` |
| `history_check_object` | `team:global_survey_platform_admins` |
| `history_check_relation` | `member` |

> Survey templates are restricted to members of the `global_survey_platform_admins` team. No FGA-sync access message is sent for templates.

### Search Behavior

| Field | Value |
|---|---|
| `fulltext` | `title nickname` (space-joined) |
| `name_and_aliases` | `title`, `nickname` (deduplicated, non-empty values only) |
| `sort_name` | `title` |
| `public` | `false` (always) |

### Parent References

_(none)_

---

## NATS Subjects

| Purpose | Subject |
|---|---|
| Index a survey | `lfx.index.survey` |
| Index a survey response | `lfx.index.survey_response` |
| Index a survey template | `lfx.index.survey_template` |
| Update FGA access tuples | `lfx.fga-sync.update_access` |
| Delete FGA access tuples | `lfx.fga-sync.delete_access` |

## KV Buckets Watched

| Bucket | Filter Subject | Resource Type |
|---|---|---|
| `v1-objects` | `$KV.v1-objects.itx-surveys.>` | Survey |
| `v1-objects` | `$KV.v1-objects.itx-survey-responses.>` | Survey Response |
| `v1-objects` | `$KV.v1-objects.surveymonkey-surveys.>` | Survey Template |

## Actions

All three resource types support the following actions:

| Action | Trigger |
|---|---|
| `created` | Key not yet present in the `v1-mappings` KV bucket |
| `updated` | Key already exists in the `v1-mappings` KV bucket, including tombstoned `!del` entries |
| `deleted` | Key deleted, purged, or `_sdc_deleted_at` present in the payload |
