# FGA Contract — Survey Service

This document is the authoritative reference for all messages the survey service sends to the fga-sync service, which writes and deletes [OpenFGA](https://openfga.dev/) relationship tuples to enforce access control.

The full OpenFGA type definitions (relations, schema) for all object types are defined in the [platform model](https://github.com/linuxfoundation/lfx-v2-helm/blob/main/charts/lfx-platform/templates/openfga/model.yaml).

**Update this document in the same PR as any change to FGA message construction.**

> **Note:** `survey_template` does not send FGA messages — it is indexed only.

---

## Object Types

- [Survey](#survey)
- [Survey Response](#survey-response)

---

## Message Format

All messages use the generic FGA message format on the following NATS subjects:

| Subject | Used for |
|---|---|
| `lfx.fga-sync.update_access` | Create and update operations |
| `lfx.fga-sync.delete_access` | Delete operations |

Each message carries `object_type`, `operation`, and a `data` map. The sections below describe the `data` contents for each object type.

---

## Survey

**Source struct:** `internal/domain/event_models.go` — `SurveyData`

**Synced on:** create, update, delete of a survey.

### Access Config

| Field | Value |
|---|---|
| `object_type` | `survey` |
| `public` | `false` (always) |

### Relations

_(none set by this service)_

### References

| Reference | Value | Condition |
|---|---|---|
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
|---|---|
| `object_type` | `survey_response` |
| `public` | `false` (always) |

### Relations

| Relation | Value | Condition |
|---|---|---|
| `owner` | `Username` (Auth0 `sub`) | Only when `Username` is non-empty |

### References

| Reference | Value | Condition |
|---|---|---|
| `survey` | `SurveyUID` | Only when `SurveyUID` is non-empty |

> The update message is skipped entirely if both `Username` and `SurveyUID` are empty.

### Delete

On delete, only `uid` is sent — all FGA tuples for `survey_response:{uid}` are removed by the fga-sync service.

---

## Triggers

| Operation | Object Type | Subject | Notes |
|---|---|---|---|
| Create survey | `survey` | `lfx.fga-sync.update_access` | Skipped if all committee and project UIDs are empty |
| Update survey | `survey` | `lfx.fga-sync.update_access` | Skipped if all committee and project UIDs are empty |
| Delete survey | `survey` | `lfx.fga-sync.delete_access` | Always sent |
| Create survey response | `survey_response` | `lfx.fga-sync.update_access` | Skipped if both `Username` and `SurveyUID` are empty |
| Update survey response | `survey_response` | `lfx.fga-sync.update_access` | Skipped if both `Username` and `SurveyUID` are empty |
| Delete survey response | `survey_response` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update/delete survey template | _(none)_ | _(none)_ | No FGA message sent |
