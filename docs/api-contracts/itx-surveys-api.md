# ITX Surveys API Contracts

This document details the API contracts for ITX survey proxy endpoints, showing both the proxy API (LFX Survey Service) and the underlying ITX API schemas.

## Overview

The LFX Survey Service proxies requests to the ITX Survey API service with the following flow:

```
Client → LFX Survey Service (Proxy) → ITX Service → SurveyMonkey API
```

**Proxy API (LFX Survey Service)**:

- Base Path: `/surveys`
- Authorization: Bearer token (JWT via Heimdall/OpenFGA)
- Content-Type: `application/json`

**ITX API (Underlying Service)**:

- Base Path: `/v2/surveys`
- Authorization: OAuth2 M2M (added automatically by proxy)
- Content-Type: `application/json`

---

## Create Survey

### Proxy API Endpoint

**Method**: `POST /surveys`

**Authorization**: Requires `writer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request Body**:

```json
{
  "project_uid": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "creator_id": "005f1000009RbC4AAK",
  "survey_title": "Q1 2024 Committee Member Satisfaction Survey",
  "survey_send_date": "2024-01-15T09:00:00Z",
  "survey_cutoff_date": "2024-01-29T23:59:59Z",
  "survey_reminder_rate_days": 7,
  "email_subject": "Please complete our Q1 2024 survey",
  "email_body": "<p>Dear committee member,<br>Please take a few minutes to complete our quarterly survey.</p>",
  "email_body_text": "Dear committee member,\n\nPlease take a few minutes to complete our quarterly survey.",
  "committees": [
    "qa1e8536-a985-4cf5-b981-a170927a1d11",
    "dc2f7654-b896-5dg6-c092-b281938b2e22"
  ],
  "committee_voting_enabled": true
}
```

**Response**: `201 Created`

```json
{
  "id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "survey_monkey_id": "123456789",
  "is_project_survey": true,
  "stage_filter": ["Active"],
  "creator_username": "jdoe",
  "creator_name": "John Doe",
  "creator_id": "005f1000009RbC4AAK",
  "created_at": "2024-01-10T08:00:00Z",
  "modified_at": "2024-01-10T08:00:00Z",
  "survey_title": "Q1 2024 Committee Member Satisfaction Survey",
  "survey_send_date": "2024-01-15T09:00:00Z",
  "survey_cutoff_date": "2024-01-29T23:59:59Z",
  "survey_reminder_rate_days": 7,
  "survey_status": "disabled",
  "project_uid": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "project_name": "Kubernetes",
  "foundation_uid": "003170000123XHTAA2",
  "foundation_name": "Cloud Native Computing Foundation",
  "email_subject": "Please complete our Q1 2024 survey",
  "email_body": "<p>Dear committee member,<br>Please take a few minutes to complete our quarterly survey.</p>",
  "email_body_text": "Dear committee member,\n\nPlease take a few minutes to complete our quarterly survey.",
  "committee_voting_enabled": true,
  "committees": [
    {
      "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
      "committee_name": "Technical Steering Committee",
      "project_id": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
      "project_name": "Kubernetes",
      "total_recipients": 15,
      "total_responses": 0,
      "nps_value": null
    }
  ],
  "total_recipients": 15,
  "total_responses": 0,
  "response_rate": 0.0,
  "nps_value": null,
  "num_promoters": 0,
  "num_passives": 0,
  "num_detractors": 0,
  "total_bounced_emails": 0,
  "num_automated_reminders_to_send": 0,
  "num_automated_reminders_sent": 0,
  "next_automated_reminder_at": null,
  "latest_automated_reminder_sent_at": null
}
```

### ITX API Endpoint

**Method**: `POST /v2/surveys/schedule`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
Content-Type: application/json
```

**Request Body**:

```json
{
  "project_id": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "creator_id": "005f1000009RbC4AAK",
  "survey_title": "Q1 2024 Committee Member Satisfaction Survey",
  "survey_send_date": "2024-01-15T09:00:00Z",
  "survey_cutoff_date": "2024-01-29T23:59:59Z",
  "survey_reminder_rate_days": 7,
  "email_subject": "Please complete our Q1 2024 survey",
  "email_body": "<p>Dear committee member,<br>Please take a few minutes to complete our quarterly survey.</p>",
  "email_body_text": "Dear committee member,\n\nPlease take a few minutes to complete our quarterly survey.",
  "committees": [
    "qa1e8536-a985-4cf5-b981-a170927a1d11",
    "dc2f7654-b896-5dg6-c092-b281938b2e22"
  ],
  "committee_voting_enabled": true
}
```

**Response**: `201 Created`

Response body is identical to Proxy API response.

### Field Mapping

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `project_uid` | `project_id` | Project identifier - field name differs |
| All other request fields | Same | Request fields are identical except project identifier |
| All response fields | Same | Response fields are identical |

---

## Get Survey

### Proxy API Endpoint

**Method**: `GET /surveys/{survey_id}`

**Authorization**: Requires `viewer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Response**: `200 OK`

Response body is identical to Create Survey response.

### ITX API Endpoint

**Method**: `GET /v2/surveys/{survey_id}/schedule`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Response**: `200 OK`

Response body is identical to ITX Create Survey response.

### Field Mapping

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `/surveys/{survey_id}` | `/v2/surveys/{survey_id}/schedule` | Path differs - proxy has shorter path |
| All response fields | Same | Response fields are identical |

---

## Update Survey

### Proxy API Endpoint

**Method**: `PUT /surveys/{survey_id}`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Request Body**:

All fields from Create Survey request are optional. Only include fields to be updated.

```json
{
  "survey_title": "Q1 2024 Committee Member Satisfaction Survey (Updated)",
  "survey_cutoff_date": "2024-02-05T23:59:59Z"
}
```

**Response**: `200 OK`

Response body is identical to Create Survey response with updated values.

**Note**: Updates are only allowed when survey status is 'disabled'.

### ITX API Endpoint

**Method**: `PUT /v2/surveys/{survey_id}/schedule`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
Content-Type: application/json
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Request Body**: Same as Proxy API

**Response**: `200 OK`

Response body is identical to ITX Create Survey response with updated values.

### Field Mapping

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `/surveys/{survey_id}` | `/v2/surveys/{survey_id}/schedule` | Path differs - proxy has shorter path |
| All fields | Same | Request/response fields are identical |

---

## Delete Survey

### Proxy API Endpoint

**Method**: `DELETE /surveys/{survey_id}`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Response**: `204 No Content`

**Note**: Deletion is only allowed when survey status is 'disabled'.

### ITX API Endpoint

**Method**: `DELETE /v2/surveys/{survey_id}/schedule`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Response**: `204 No Content`

### Field Mapping

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `/surveys/{survey_id}` | `/v2/surveys/{survey_id}/schedule` | Path differs - proxy has shorter path |

---

## Bulk Resend Survey

### Proxy API Endpoint

**Method**: `POST /surveys/{survey_id}/bulk_resend`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Request Body**:

```json
{
  "recipient_ids": [
    "cba14f40-1636-11ec-9621-0242ac130002",
    "cba14f40-1636-11ec-9621-0242ac130003"
  ]
}
```

**Response**: `204 No Content`

### ITX API Endpoint

**Method**: `POST /v2/surveys/{survey_id}/bulk_resend`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
Content-Type: application/json
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Request Body**: Identical to Proxy API

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Preview Send

### Proxy API Endpoint

**Method**: `GET /surveys/{survey_id}/preview_send`

**Authorization**: Requires `viewer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Query Parameters**:

- `committee_id` (string, optional) - Filter preview by specific committee

**Response**: `200 OK`

```json
{
  "affected_projects": [
    {
      "id": "003170000123XHTAA2",
      "name": "Express JS",
      "slug": "express-gateway",
      "status": "Active",
      "logo_url": "https://example.com/logo.png"
    }
  ],
  "affected_committees": [
    {
      "project_id": "003170000123XHTAA2",
      "project_name": "Kubernetes",
      "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
      "committee_name": "Technical Steering Committee",
      "committee_category": "Technical Steering Committee"
    }
  ],
  "affected_recipients": [
    {
      "user_id": "005f1000009RbC4AAK",
      "name": "John Doe",
      "first_name": "John",
      "last_name": "Doe",
      "username": "jdoe",
      "email": "john.doe@example.com",
      "role": "Voting Rep"
    }
  ]
}
```

### ITX API Endpoint

**Method**: `GET /v2/surveys/{survey_id}/preview_send`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Query Parameters**: Same as Proxy API

**Response**: `200 OK`

Response body is identical to Proxy API response.

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Send Missing Recipients

### Proxy API Endpoint

**Method**: `POST /surveys/{survey_id}/send_missing_recipients`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Query Parameters**:

- `committee_id` (string, optional) - Resync only specified committee

**Response**: `204 No Content`

**Note**: This endpoint sends survey emails to committee members who haven't received the survey yet (e.g., members added after survey was sent).

### ITX API Endpoint

**Method**: `POST /v2/surveys/{survey_id}/send_missing_recipients`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Query Parameters**: Same as Proxy API

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Delete Recipient Group

### Proxy API Endpoint

**Method**: `DELETE /surveys/{survey_id}/recipient_group`

**Authorization**: Requires `writer` permission on the survey

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Query Parameters**:

- `committee_id` (string, optional) - Committee ID to remove
- `project_id` (string, optional) - Project ID to remove
- `foundation_id` (string, optional) - Foundation ID (removes all subprojects)

**Response**: `204 No Content`

**Note**: Removes a recipient group (committee, project, or foundation) from the survey and recalculates statistics.

### ITX API Endpoint

**Method**: `DELETE /v2/surveys/{survey_id}/recipient_group`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `survey_id` (string, required) - Survey identifier

**Query Parameters**: Same as Proxy API

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Validate Email

### Proxy API Endpoint

**Method**: `POST /surveys/validate_email`

**Authorization**: Requires `writer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request Body**:

```json
{
  "body": "An example survey body with the quarter {{quarter}}",
  "subject": "An example survey subject with the year {{year}}"
}
```

**Note**: Both fields are optional. The endpoint validates email template variables and returns the processed templates with variables replaced.

**Response**: `200 OK`

```json
{
  "body": "An example survey body with the quarter Q1",
  "subject": "An example survey subject with the year 2023"
}
```

### ITX API Endpoint

**Method**: `POST /v2/surveys/validate_email`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
Content-Type: application/json
```

**Request Body**: Identical to Proxy API

**Response**: `200 OK`

Response body is identical to Proxy API response.

### Field Mapping

All fields are identical between Proxy and ITX API.

### Template Variables

The email validation endpoint supports various template variables that can be used in email templates:

| Variable | Description | Example Output |
|----------|-------------|----------------|
| `{{quarter}}` | Current quarter | Q1, Q2, Q3, Q4 |
| `{{year}}` | Current year | 2024 |
| `{{project_name}}` | Project name | Kubernetes |
| `{{committee_name}}` | Committee name | Technical Steering Committee |
| `{{recipient_name}}` | Recipient's full name | John Doe |
| `{{survey_link}}` | Unique survey link | <https://surveymonkey.com/>... |

**Note**: The actual template variables supported may vary. This endpoint allows you to test templates before sending surveys to ensure variables are correctly replaced.

---

## Common Data Types

### SurveyCommittee Object

**Structure is identical in both APIs**:

```json
{
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "committee_name": "Technical Steering Committee",
  "project_id": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "project_name": "Kubernetes",
  "total_recipients": 15,
  "total_responses": 8,
  "nps_value": 75.5
}
```

### LFXProject Object

**Structure is identical in both APIs**:

```json
{
  "id": "003170000123XHTAA2",
  "name": "Express JS",
  "slug": "express-gateway",
  "status": "Active",
  "logo_url": "https://example.com/logo.png"
}
```

### ExcludedCommittee Object

**Structure is identical in both APIs**:

```json
{
  "project_id": "003170000123XHTAA2",
  "project_name": "Kubernetes",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "committee_name": "Technical Steering Committee",
  "committee_category": "Technical Steering Committee"
}
```

### ITXPreviewRecipient Object

**Structure is identical in both APIs**:

```json
{
  "user_id": "005f1000009RbC4AAK",
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "username": "jdoe",
  "email": "john.doe@example.com",
  "role": "Voting Rep"
}
```

---

## Summary of Key Differences

| Aspect | Proxy API (LFX) | ITX API |
|--------|-----------------|---------|
| **Base Path** | `/surveys` | `/v2/surveys` |
| **Authentication** | JWT Bearer token | OAuth2 M2M token |
| **Authorization** | Heimdall/OpenFGA | Handled by ITX service |
| **Create Endpoint** | `POST /surveys` | `POST /v2/surveys/schedule` |
| **Get Endpoint** | `GET /surveys/{id}` | `GET /v2/surveys/{id}/schedule` |
| **Update Endpoint** | `PUT /surveys/{id}` | `PUT /v2/surveys/{id}/schedule` |
| **Delete Endpoint** | `DELETE /surveys/{id}` | `DELETE /v2/surveys/{id}/schedule` |
| **Project Field** | `project_uid` (request only) | `project_id` (request only) |
| **Required Header** | `Authorization: Bearer <jwt>` | `Authorization: Bearer <oauth2>` |

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
  "code": "400",
  "message": "Invalid survey_send_date: must be in the future"
}
```
