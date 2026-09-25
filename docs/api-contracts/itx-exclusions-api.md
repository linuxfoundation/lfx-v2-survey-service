# ITX Exclusions API Contracts

This document details the API contracts for ITX survey exclusion proxy endpoints, showing both the proxy API (LFX Survey Service) and the underlying ITX API schemas.

## Overview

The LFX Survey Service proxies requests to the ITX Exclusions API service with the following flow:

```
Client → LFX Survey Service (Proxy) → ITX Service
```

**Proxy API (LFX Survey Service)**:

- Base Path: `/surveys/exclusion`
- Authorization: Bearer token (JWT via Heimdall/OpenFGA)
- Content-Type: `application/json`

**ITX API (Underlying Service)**:

- Base Path: `/v2/surveys/exclusion`
- Authorization: OAuth2 M2M (added automatically by proxy)
- Content-Type: `application/json`

---

## Create Exclusion

### Proxy API Endpoint

**Method**: `POST /surveys/exclusion`

**Authorization**: Requires `writer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request Body**:

```json
{
  "email": "user@example.com",
  "user_id": "005f1000009RbC4AAK",
  "survey_id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "global_exclusion": "true"
}
```

**Note**: All fields are optional. Provide `email` or `user_id` to identify the user to exclude. Set `global_exclusion` to "true" for global exclusions (across all surveys) or provide `survey_id` for survey-specific exclusions.

**Response**: `201 Created`

```json
{
  "id": "12345",
  "email": "user@example.com",
  "survey_id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "global_exclusion": "true",
  "user_id": "005f1000009RbC4AAK"
}
```

### ITX API Endpoint

**Method**: `POST /v2/surveys/exclusion`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
Content-Type: application/json
```

**Request Body**: Identical to Proxy API

**Response**: `201 Created`

Response body is identical to Proxy API response.

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Delete Exclusion

### Proxy API Endpoint

**Method**: `DELETE /surveys/exclusion`

**Authorization**: Requires `writer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request Body**:

```json
{
  "email": "user@example.com",
  "user_id": "005f1000009RbC4AAK",
  "survey_id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "global_exclusion": "true"
}
```

**Note**: All fields are optional. Use the same criteria that were used to create the exclusion.

**Response**: `204 No Content`

### ITX API Endpoint

**Method**: `DELETE /v2/surveys/exclusion`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
Content-Type: application/json
```

**Request Body**: Identical to Proxy API

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Get Exclusion

### Proxy API Endpoint

**Method**: `GET /surveys/exclusion/{exclusion_id}`

**Authorization**: Requires `viewer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `exclusion_id` (string, required) - Exclusion identifier

**Response**: `200 OK`

```json
{
  "id": "12345",
  "email": "user@example.com",
  "survey_id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "global_exclusion": "true",
  "user_id": "005f1000009RbC4AAK",
  "user": {
    "id": "005f1000009RbC4AAK",
    "username": "jdoe",
    "emails": [
      {
        "id": "email123",
        "email_address": "user@example.com",
        "is_primary": true
      }
    ]
  }
}
```

### ITX API Endpoint

**Method**: `GET /v2/surveys/exclusion/{exclusion_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `exclusion_id` (string, required) - Exclusion identifier

**Response**: `200 OK`

Response body is identical to Proxy API response.

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Delete Exclusion by ID

### Proxy API Endpoint

**Method**: `DELETE /surveys/exclusion/{exclusion_id}`

**Authorization**: Requires `writer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `exclusion_id` (string, required) - Exclusion identifier

**Response**: `204 No Content`

### ITX API Endpoint

**Method**: `DELETE /v2/surveys/exclusion/{exclusion_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
```

**Path Parameters**:

- `exclusion_id` (string, required) - Exclusion identifier

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Common Data Types

### Exclusion Object

**Structure is identical in both APIs**:

```json
{
  "id": "12345",
  "email": "user@example.com",
  "survey_id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "global_exclusion": "true",
  "user_id": "005f1000009RbC4AAK"
}
```

### ExtendedExclusion Object (with User Info)

**Structure is identical in both APIs**:

```json
{
  "id": "12345",
  "email": "user@example.com",
  "survey_id": "b03cdbaf-53b1-4d47-bc04-dd7e459dd309",
  "committee_id": "qa1e8536-a985-4cf5-b981-a170927a1d11",
  "global_exclusion": "true",
  "user_id": "005f1000009RbC4AAK",
  "user": {
    "id": "005f1000009RbC4AAK",
    "username": "jdoe",
    "emails": [
      {
        "id": "email123",
        "email_address": "user@example.com",
        "is_primary": true
      }
    ]
  }
}
```

---

## Summary of Key Differences

| Aspect | Proxy API (LFX) | ITX API |
|--------|-----------------|---------|
| **Base Path** | `/surveys/exclusion` | `/v2/surveys/exclusion` |
| **Authentication** | JWT Bearer token | OAuth2 M2M token |
| **Authorization** | Heimdall/OpenFGA | Handled by ITX service |
| **Required Header** | `Authorization: Bearer <jwt>` | `Authorization: Bearer <oauth2>` |

**Note**: Unlike other survey endpoints, exclusion endpoints have identical paths in both Proxy and ITX APIs (only base path differs).

---

## Error Responses

Both APIs return similar error structures:

**HTTP Status Codes**:

- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found (GET only)
- `500 Internal Server Error` - Server error

**Error Response Body**:

```json
{
  "code": "400",
  "message": "Either email or user_id must be provided"
}
```
