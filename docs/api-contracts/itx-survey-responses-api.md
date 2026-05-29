# ITX Survey Responses API Contracts

This document details the API contracts for ITX survey response proxy endpoints, showing both the proxy API (LFX Survey Service) and the underlying ITX API schemas.

## Overview

The LFX Survey Service proxies requests to the ITX Survey Response API service with the following flow:

```
Client → LFX Survey Service (Proxy) → ITX Service → SurveyMonkey API
```

**Proxy API (LFX Survey Service)**:

- Base Path: `/surveys/{survey_id}/responses`
- Authorization: Bearer token (JWT via Heimdall/OpenFGA)
- Content-Type: `application/json`

**ITX API (Underlying Service)**:

- Base Path: `/v2/surveys/{survey_id}/responses`
- Authorization: OAuth2 M2M (added automatically by proxy)
- Content-Type: `application/json`

---

## Delete Survey Response

### Proxy API Endpoint

**Method**: `DELETE /surveys/{survey_id}/responses/{response_id}`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier
- `response_id` (string, required) - Response/recipient identifier

**Response**: `204 No Content`

**Note**: This removes the recipient from the survey and recalculates survey statistics.

### ITX API Endpoint

**Method**: `DELETE /v2/surveys/{survey_id}/responses/{response_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier
- `response_id` (string, required) - Response/recipient identifier

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Resend Survey Response

### Proxy API Endpoint

**Method**: `POST /surveys/{survey_id}/responses/{response_id}/resend`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier
- `response_id` (string, required) - Response/recipient identifier

**Response**: `204 No Content`

**Note**: This resends the survey email to a specific user.

### ITX API Endpoint

**Method**: `POST /v2/surveys/{survey_id}/responses/{response_id}/resend`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier
- `response_id` (string, required) - Response/recipient identifier

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Summary of Key Differences

| Aspect | Proxy API (LFX) | ITX API |
|--------|-----------------|---------|
| **Base Path** | `/surveys/{survey_id}/responses` | `/v2/surveys/{survey_id}/responses` |
| **Authentication** | JWT Bearer token | OAuth2 M2M token |
| **Authorization** | Heimdall/OpenFGA | Handled by ITX service |
| **Required Header** | `Authorization: Bearer <jwt>` | `Authorization: Bearer <oauth2>` |

---

## List Survey Responses

Returns a paginated list of individual per-recipient responses for a survey. Each response includes the
recipient's identity, their submitted answers, NPS score, and SES delivery tracking fields.

### Proxy API Endpoint

**Method**: `GET /surveys/{survey_uid}/responses`

**Authorization**: Requires JWT with `manage:projects` + `manage:surveys` scopes

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_uid` (string, required) — Survey identifier (V2 UUID)

**Query Parameters** (all optional):

- `page_token` (string) — Opaque pagination token; omit for the first page; pass the `meta.page_token` from the previous page for subsequent pages
- `per_page` (string) — Maximum number of responses per page (e.g. `"25"`)
- `project_uid` (string) — LFX Project UID (V2) to filter responses to a single participating project; mapped to V1 SFID before forwarding
- `project_uids` (string) — Comma-delimited list of LFX Project UIDs (V2) to filter responses; each mapped to V1 before forwarding

**Response**: `200 OK`

```json
{
  "data": [
    {
      "id": "cba14f40-1636-11ec-9621-0242ac130002",
      "survey_uid": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
      "survey_link": "https://surveymonkey.com/r/abc123",
      "committee_uid": "qa1e8536-a985-4cf5-b981-a170927a1d11",
      "email": "jane.doe@example.com",
      "first_name": "Jane",
      "last_name": "Doe",
      "username": "jdoe",
      "role": "Voting Rep",
      "job_title": "Principal Engineer",
      "membership_tier": "Platinum",
      "voting_status": "Eligible",
      "organization": { "id": "org-001", "name": "Acme Corp" },
      "project": { "uid": "qa1e8536-a985-4cf5-b981-a170927a1d11", "name": "Kubernetes" },
      "response_status": "Responded",
      "created_at": "2026-01-15T09:00:00Z",
      "response_datetime": "2026-01-20T14:32:00Z",
      "last_received_time": "2026-01-15T09:00:05Z",
      "num_automated_reminders_received": 1,
      "nps_value": 9.0,
      "survey_monkey_respondent_id": "12345678",
      "survey_monkey_question_answers": [
        {
          "question_id": "q-001",
          "question_text": "How satisfied are you with project governance?",
          "question_family": "rating",
          "question_subtype": "ranking",
          "answers": [
            { "choice_id": "c-001", "text": "Very satisfied" }
          ]
        }
      ],
      "ses_delivery_successful": true,
      "ses_email_opened": true,
      "ses_email_opened_last_time": "2026-01-16T10:00:00Z",
      "ses_link_clicked": true,
      "ses_link_clicked_last_time": "2026-01-16T10:05:00Z"
    }
  ],
  "meta": {
    "page_token": "",
    "total_pages": 1,
    "total_results": 1,
    "per_page": 25
  }
}
```

**Note**: An empty string `page_token` in the response `meta` indicates the last page.

### ITX API Endpoint

**Method**: `GET /v2/surveys/{survey_id}/responses`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) — Survey identifier passed through unchanged

**Query Parameters**: `page_token`, `per_page`, `project_id` (V1 SFID), `project_ids` (comma-delimited V1 SFIDs)

**Response**: `200 OK` — same `PaginatedSurveyResponses` shape as above

### Field Mapping (Proxy → ITX)

| Proxy query param | ITX query param | Transformation |
|---|---|---|
| `project_uid` (V2 UUID) | `project_id` (V1 SFID) | V2→V1 via NATS ID mapper |
| `project_uids` (comma V2 UUIDs) | `project_ids` (comma V1 SFIDs) | each V2→V1 via NATS ID mapper |
| `page_token` | `page_token` | passthrough |
| `per_page` | `per_page` | passthrough |

### Field Mapping (ITX response → Proxy response)

| ITX field | Proxy field | Transformation |
|---|---|---|
| `project.id` (V1 SFID) | `project.uid` (V2 UUID) | V1→V2 via NATS ID mapper; falls back to V1 on failure |
| `committee_id` (V1 SFID) | `committee_uid` (V2 UUID) | V1→V2 via NATS ID mapper; falls back to V1 on failure |
| All other fields | identical | passthrough |

---

## Error Responses

Both APIs return similar error structures:

**HTTP Status Codes**:

- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

**Error Response Body**:

```json
{
  "code": "404",
  "message": "Survey response not found"
}
```
