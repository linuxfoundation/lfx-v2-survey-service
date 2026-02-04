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
