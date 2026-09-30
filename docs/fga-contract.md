# FGA Contract — Survey Service

This document is the authoritative reference for all messages the survey service sends to the fga-sync service, which writes and deletes [OpenFGA](https://openfga.dev/) relationship tuples to enforce access control.

The full OpenFGA type definitions (relations, schema) for all object types are defined in the [platform model](https://github.com/linuxfoundation/lfx-v2-helm/blob/main/charts/lfx-platform/templates/openfga/model.yaml).

**Update this document in the same PR as any change to FGA message construction.**

> **Note:** `survey_template` does not send FGA messages — it is indexed only.

---

## Prerequisites

> **Deployment order:** `fga-sync` must be updated to accept LFX usernames in relation values (e.g., `owner`) before this service version is deployed. See [LFXV2-1962](https://linuxfoundation.atlassian.net/browse/LFXV2-1962).

> **Username handling:** This service forwards the v1 `username` field unchanged when it passes LFX username format validation (`^[a-zA-Z0-9._-]+$`). Invalid values are logged and omitted from the FGA `owner` relation. fga-sync builds OpenFGA user principals as `user:{username}` without additional sanitization. For email-only invitations (no usable `username`), the owner principal is `user:{username}` for the LFX username resolved from the invitation email via the auth-service `email_to_username` lookup — see [Survey Response](#survey-response).

---

## Object Types

- [Survey](#survey)
- [Survey Response](#survey-response)

---

## Message Format

All messages use the generic FGA message format on the following NATS subjects:

| Subject | Used for |
| --- | --- |
| `lfx.fga-sync.update_access` | Create and update operations |
| `lfx.fga-sync.delete_access` | Delete operations |

Each message carries `object_type`, `operation`, and a `data` map. The sections below describe the `data` contents for each object type.

---

## Survey

**Source struct:** `internal/domain/event_models.go` — `SurveyData`

**Synced on:** create, update, delete of a survey.

### Access Config

| Field | Value |
| --- | --- |
| `object_type` | `survey` |
| `public` | `false` (always) |

### Relations

_(none set by this service)_

### References

| Reference | Value | Condition |
| --- | --- | --- |
| `committee` | `CommitteeUID` | One entry per committee in `Committees` where `CommitteeUID` is non-empty |
| `project` | `ProjectUID` | One entry per unique project across all committees (deduplicated); omitted when empty |

> The update message is skipped entirely if all committee and project UIDs are empty.

### Delete

On delete, only `uid` is sent — all FGA tuples for `survey:{uid}` are removed by the fga-sync service.

---

## Survey Response

**Source struct:** `internal/domain/event_models.go` — `SurveyResponseData`

**Synced on:** create, update, delete of a survey response.

### Access Config

| Field | Value |
| --- | --- |
| `object_type` | `survey_response` |
| `public` | `false` (always) |

### Relations

| Relation | Value | Condition |
| --- | --- | --- |
| `owner` | LFX username (from v1 `username` field) | `Username` is non-empty and passes LFX username format validation |
| `owner` | LFX username resolved from the invitation email via the auth-service `lfx.auth-service.email_to_username` request/reply | No owner was resolved from `Username` (empty or invalid), `Email` is non-empty, and the email resolves to an existing account whose username passes LFX username format validation |

### References

| Reference | Value | Condition |
| --- | --- | --- |
| `survey` | `SurveyUID` | Only when `SurveyUID` is non-empty |

> The update message is skipped entirely when no relations and no references would be sent (no resolvable owner and an empty `SurveyUID`).
>
> **Ownership preservation:** fga-sync's `update_access` is a destructive full sync — a relation absent from the payload has its live tuples deleted. Whenever the message is sent without an `owner` relation, it carries `exclude_relations: ["owner"]` so a previously granted owner tuple survives re-sends and edits. A preserved owner is only revoked by deleting the response (fga-sync `delete_access`) or by an operator deleting the tuple directly in OpenFGA.
>
> **Unresolvable emails:** the auth-service lookup searches the account's primary email first and then retries linked alternate emails (`EmailToUsername` delegates to `searchByEmailWithFallback`); only an invitation addressed to an email with no linked account never resolves to an owner. No access-model exception is made for account-less invitees — the platform's LFID invite conversion flow remains their path to access.

### Delete

On delete, only `uid` is sent — all FGA tuples for `survey_response:{uid}` are removed by the fga-sync service.

---

## Triggers

| Operation | Object Type | Subject | Notes |
| --- | --- | --- | --- |
| Create survey | `survey` | `lfx.fga-sync.update_access` | Skipped if all committee and project UIDs are empty |
| Update survey | `survey` | `lfx.fga-sync.update_access` | Skipped if all committee and project UIDs are empty |
| Delete survey | `survey` | `lfx.fga-sync.delete_access` | Always sent |
| Create survey response | `survey_response` | `lfx.fga-sync.update_access` | Skipped when no owner resolves and `SurveyUID` is empty; carries `exclude_relations: ["owner"]` when no owner resolves |
| Update survey response | `survey_response` | `lfx.fga-sync.update_access` | Skipped when no owner resolves and `SurveyUID` is empty; carries `exclude_relations: ["owner"]` when no owner resolves |
| Delete survey response | `survey_response` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update/delete survey template | _(none)_ | _(none)_ | No FGA message sent |
