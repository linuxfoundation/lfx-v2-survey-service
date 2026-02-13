# ITX Proxy Implementation Architecture

This document describes how the ITX proxy endpoints are implemented in the codebase and the architectural patterns used.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Code Organization](#code-organization)
- [Implementation Patterns](#implementation-patterns)
- [Data Flow](#data-flow)
- [Key Components](#key-components)
- [Field Mapping](#field-mapping)
- [Error Handling](#error-handling)
- [Configuration](#configuration)

---

## Overview

The LFX Survey Service acts as a lightweight proxy to the ITX Survey API service, providing:

1. **Authentication Translation** - JWT (Heimdall) → OAuth2 M2M (Auth0)
2. **Authorization** - OpenFGA fine-grained access control
3. **ID Mapping** - V2 UUIDs → V1 Salesforce IDs (via NATS)
4. **Field Mapping** - LFX v2 conventions → ITX conventions
5. **Stateless Proxy** - No local persistence, all data managed by ITX

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    LFX Survey Service                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Survey Endpoints                            │   │
│  │              /surveys/*                                  │   │
│  └────────────────────┬─────────────────────────────────────┘   │
│                       │                                         │
│                       ▼                                         │
│  ┌────────────────────────────────────────────────────────┐     │
│  │           Service Layer (Proxy Logic)                  │     │
│  │  - JWT Authentication via Heimdall                     │     │
│  │  - ID mapping (V2 UIDs ↔ V1 SFIDs via NATS)          │     │
│  │  - Field mapping (committee_uid → committees array)   │     │
│  │  - Request/response transformation                     │     │
│  └────────────────────┬───────────────────────────────────┘     │
│                       │                                         │
│                       ▼                                         │
│  ┌────────────────────────────────────────────────────────┐     │
│  │         ITX Proxy Client (HTTP Client)                 │     │
│  │  - OAuth2 M2M authentication with Auth0               │     │
│  │  - HTTP requests to ITX service                       │     │
│  │  - Error mapping                                       │     │
│  └────────────────────┬───────────────────────────────────┘     │
│                       │                                         │
└───────────────────────┼─────────────────────────────────────────┘
                        ▼
              ┌──────────────────┐
              │   ITX Service    │
              │  (OAuth2 M2M)    │
              └────────┬─────────┘
                       ▼
              ┌──────────────────┐
              │ SurveyMonkey API │
              └──────────────────┘
```

---

## Code Organization

### Directory Structure

```
cmd/survey-api/
├── main.go                      # Service entry point
└── api.go                       # Goa handler implementations

api/survey/v1/design/
├── survey.go                    # Goa API design (DSL)
└── types.go                     # Goa type definitions

internal/
├── domain/
│   ├── auth.go                  # Authentication interface
│   ├── idmapper.go             # ID mapping interface (v1 ↔ v2)
│   ├── proxy.go                # ITX proxy client interface
│   └── errors.go               # Domain error types
├── service/
│   ├── survey_service.go       # Survey business logic
│   ├── survey_response_service.go  # Survey response business logic
│   └── mappers.go              # Domain ↔ Goa converters
└── infrastructure/
    ├── auth/
    │   └── jwt_auth.go         # JWT authentication implementation
    ├── idmapper/
    │   └── nats_mapper.go      # NATS-based ID mapping
    └── proxy/
        └── itx_client.go       # ITX HTTP proxy client

pkg/
├── constants/                   # Shared constants
└── models/itx/
    └── models.go               # ITX request/response models

gen/
└── ...                         # Generated Goa code
```

---

## Implementation Patterns

### API Handler Pattern

**File**: [cmd/survey-api/api.go](../cmd/survey-api/api.go)

```go
// SurveyAPI implements the survey.Service interface
type SurveyAPI struct {
    surveyService *service.SurveyService
}

// ScheduleSurvey handles POST /surveys
func (api *SurveyAPI) ScheduleSurvey(ctx context.Context, p *survey.ScheduleSurveyPayload) (*survey.SurveyScheduleResult, error) {
    // Delegate to service layer
    return api.surveyService.ScheduleSurvey(ctx, p)
}
```

**Pattern**: Thin handler that delegates to service layer

### Service Layer Pattern

**File**: [internal/service/survey_service.go](../internal/service/survey_service.go)

```go
func (s *SurveyService) ScheduleSurvey(ctx context.Context, p *survey.ScheduleSurveyPayload) (*survey.SurveyScheduleResult, error) {
    // 1. Parse JWT and extract principal
    principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
    if err != nil {
        return nil, &survey.UnauthorizedError{...}
    }

    // 2. Map v2 committee UID to v1 committee SFID (via NATS)
    committeeV1, err := s.idMapper.MapCommitteeV2ToV1(ctx, p.CommitteeUID)
    if err != nil {
        return nil, mapDomainError(err)
    }

    // 3. Build ITX request (field mapping: committee_uid → committees array)
    committees := []string{committeeV1}
    itxRequest := &itx.ScheduleSurveyRequest{
        Committees:             committees,
        CreatorID:              p.CreatorID,
        SurveyTitle:            p.SurveyTitle,
        // ... other fields are identical
    }

    // 4. Call ITX proxy client
    itxResponse, err := s.proxy.ScheduleSurvey(ctx, itxRequest)
    if err != nil {
        return nil, mapDomainError(err)
    }

    // 5. Convert ITX response to Goa result (maps V1 IDs back to V2 UIDs)
    result, err := s.mapITXResponseToResult(ctx, itxResponse)
    if err != nil {
        return nil, mapDomainError(err)
    }

    return result, nil
}
```

**Pattern**: Service layer handles authentication, ID mapping, field transformation, and error mapping

### Proxy Client Pattern

**File**: [internal/infrastructure/proxy/itx_client.go](../internal/infrastructure/proxy/itx_client.go)

```go
func (c *Client) ScheduleSurvey(ctx context.Context, req *itx.SurveyScheduleRequest) (*itx.SurveyScheduleResponse, error) {
    // 1. Marshal request to JSON
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // 2. Create HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v2/surveys/schedule", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }

    // 3. Add headers (OAuth2 token added automatically by transport)
    httpReq.Header.Set("Content-Type", "application/json")

    // 4. Execute request
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, domain.NewServiceUnavailableError("ITX service unavailable")
    }
    defer resp.Body.Close()

    // 5. Map HTTP errors to domain errors
    if resp.StatusCode != http.StatusCreated {
        return nil, c.mapHTTPError(resp)
    }

    // 6. Parse response
    var result itx.SurveyScheduleResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}
```

**Pattern**: HTTP client with automatic OAuth2 authentication and error mapping

---

## Data Flow

### Survey Creation Flow

```
1. Client Request
   POST /surveys
   Authorization: Bearer <jwt_token>
   {
     "committee_uid": "v2-committee-uuid",
     "survey_title": "Q1 Survey",
     ...
   }
   ↓
2. Heimdall Authorization
   - Validates JWT
   - Checks OpenFGA: user has "writer" permission on committee
   - Adds JWT to context
   ↓
3. API Handler (api.go)
   ScheduleSurvey()
   ↓
4. Service Layer (survey_service.go)
   ScheduleSurvey()
   ├─→ Parse JWT and extract principal
   ├─→ Map v2 committee UID to v1 committee SFID (via NATS)
   ├─→ Build ITX request (field mapping: committee_uid → committees array)
   └─→ Call proxy client
   ↓
5. Proxy Client (infrastructure/proxy/itx_client.go)
   ScheduleSurvey()
   ├─→ Marshal request to JSON
   ├─→ HTTP POST to ITX service
   ├─→ Add OAuth2 M2M token (automatic via transport)
   └─→ Parse response
   ↓
6. ITX Service
   POST /v2/surveys/schedule
   Authorization: Bearer <oauth2_m2m_token>
   {
     "committees": ["v1-committee-sfid"],
     "survey_title": "Q1 Survey",
     ...
   }
   ↓
7. SurveyMonkey API
   Creates survey
   ↓
8. Response flows back
   ↓
9. Service Layer
   - Converts ITX response to Goa result
   - Maps V1 committee/project SFIDs back to V2 UIDs
   ↓
10. API Response
    201 Created
    {
      "id": "survey-uuid",
      "committees": [{
        "committee_uid": "v2-committee-uuid",
        "project_uid": "v2-project-uuid",
        ...
      }],
      "survey_title": "Q1 Survey",
      ...
    }
```

---

## Key Components

### 1. Authentication Layer

**Interface**: [internal/domain/auth.go](../internal/domain/auth.go)

```go
type AuthenticationService interface {
    // ParsePrincipal validates JWT and extracts user info
    ParsePrincipal(ctx context.Context, token string, logger *slog.Logger) (string, error)
}
```

**Implementation**: [internal/infrastructure/auth/jwt_auth.go](../internal/infrastructure/auth/jwt_auth.go)

- Validates JWT using JWKS from Heimdall
- Extracts principal (username) from token
- Supports mock authentication for local development

### 2. ID Mapper Layer

**Interface**: [internal/domain/idmapper.go](../internal/domain/idmapper.go)

```go
type IDMapper interface {
    // MapCommitteeV2ToV1 maps LFX v2 committee UID to v1 Salesforce ID
    MapCommitteeV2ToV1(ctx context.Context, v2UID string) (string, error)

    // MapCommitteeV1ToV2 maps v1 committee SFID to LFX v2 UID
    MapCommitteeV1ToV2(ctx context.Context, v1SFID string) (string, error)

    // MapProjectV2ToV1 maps LFX v2 project UID to v1 Salesforce ID
    MapProjectV2ToV1(ctx context.Context, v2UID string) (string, error)

    // MapProjectV1ToV2 maps v1 project SFID to LFX v2 UID
    MapProjectV1ToV2(ctx context.Context, v1SFID string) (string, error)
}
```

**Implementation**: [internal/infrastructure/idmapper/nats_mapper.go](../internal/infrastructure/idmapper/nats_mapper.go)

- Uses NATS request/reply pattern
- Can be disabled for local development

### 3. Proxy Client Layer

**Interface**: [internal/domain/proxy.go](../internal/domain/proxy.go)

```go
type SurveyClient interface {
    ScheduleSurvey(ctx context.Context, req *itx.SurveyScheduleRequest) (*itx.SurveyScheduleResponse, error)
    GetSurvey(ctx context.Context, surveyID string) (*itx.SurveyScheduleResponse, error)
    UpdateSurvey(ctx context.Context, surveyID string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error)
    DeleteSurvey(ctx context.Context, surveyID string) error
    BulkResendSurvey(ctx context.Context, surveyID string, req *itx.BulkResendRequest) error
    // ... more methods
}

type SurveyResponseClient interface {
    DeleteResponse(ctx context.Context, surveyID string, responseID string) error
    ResendResponse(ctx context.Context, surveyID string, responseID string) error
}

type ITXProxyClient interface {
    SurveyClient
    SurveyResponseClient
}
```

**Implementation**: [internal/infrastructure/proxy/itx_client.go](../internal/infrastructure/proxy/itx_client.go)

- HTTP client with OAuth2 M2M authentication
- Automatic token refresh
- Error mapping from HTTP status codes to domain errors

---

## Field Mapping

### Request Field Mapping (Proxy → ITX)

Field differences between Proxy API and ITX API:

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `committee_uid` (single string) | `committees` (array of strings) | Proxy accepts single committee UID, converted to array for ITX |
| Committee/Project values | Mapped values | V2 UUIDs → V1 Salesforce IDs (mapped via NATS) |
| All other fields | Same | Identical field names |

**Example**:

```go
// Proxy API request
{
  "committee_uid": "qa1e8536-a985-4cf5-b981-a170927a1d11",  // V2 UUID (single)
  "survey_title": "Q1 Survey"
}

// After ID mapping and field conversion
// ITX API request
{
  "committees": ["a0C17000000abcDEF"],  // V1 Salesforce ID (array)
  "survey_title": "Q1 Survey"
}
```

### Response Field Mapping (ITX → Proxy)

Response IDs are mapped from V1 to V2:

| ITX API Response | Proxy API Response | Notes |
|-----------------|-------------------|-------|
| `committee_id` | `committee_uid` | V1 Salesforce ID → V2 UUID (mapped via NATS) |
| `project_id` | `project_uid` | V1 Salesforce ID → V2 UUID (mapped via NATS) |
| All other fields | Same | Identical field names |

**Fallback Strategy**: If V1→V2 mapping fails, the service falls back to returning V1 IDs with warning logs rather than failing the request.

### Path Mapping

| Proxy API Endpoint | ITX API Endpoint |
|-------------------|------------------|
| `POST /surveys` | `POST /v2/surveys/schedule` |
| `GET /surveys/{id}` | `GET /v2/surveys/{id}/schedule` |
| `PUT /surveys/{id}` | `PUT /v2/surveys/{id}/schedule` |
| `DELETE /surveys/{id}` | `DELETE /v2/surveys/{id}/schedule` |
| `POST /surveys/{id}/bulk_resend` | `POST /v2/surveys/{id}/bulk_resend` |
| `GET /surveys/{id}/preview_send` | `GET /v2/surveys/{id}/preview_send` |
| `POST /surveys/{id}/send_missing_recipients` | `POST /v2/surveys/{id}/send_missing_recipients` |
| `DELETE /surveys/{id}/recipient_group` | `DELETE /v2/surveys/{id}/recipient_group` |
| `DELETE /surveys/{id}/responses/{rid}` | `DELETE /v2/surveys/{id}/responses/{rid}` |
| `POST /surveys/{id}/responses/{rid}/resend` | `POST /v2/surveys/{id}/responses/{rid}/resend` |
| `POST /surveys/exclusion` | `POST /v2/surveys/exclusion` |
| `DELETE /surveys/exclusion` | `DELETE /v2/surveys/exclusion` |
| `GET /surveys/exclusion/{id}` | `GET /v2/surveys/exclusion/{id}` |
| `DELETE /surveys/exclusion/{id}` | `DELETE /v2/surveys/exclusion/{id}` |
| `POST /surveys/validate_email` | `POST /v2/surveys/validate_email` |

**Pattern**: Proxy paths are shorter (no `/schedule` suffix for CRUD operations)

---

## Error Handling

### Domain Error Types

**File**: [internal/domain/errors.go](../internal/domain/errors.go)

```go
type DomainError struct {
    Code       string
    Message    string
    StatusCode int
}

// Error constructors
func NewBadRequestError(message string) *DomainError
func NewUnauthorizedError(message string) *DomainError
func NewForbiddenError(message string) *DomainError
func NewNotFoundError(message string) *DomainError
func NewConflictError(message string) *DomainError
func NewInternalServerError(message string) *DomainError
func NewServiceUnavailableError(message string) *DomainError
```

### HTTP to Domain Error Mapping

**File**: [internal/infrastructure/proxy/itx_client.go](../internal/infrastructure/proxy/itx_client.go)

```go
func (c *Client) mapHTTPError(resp *http.Response) error {
    switch resp.StatusCode {
    case http.StatusBadRequest:
        return domain.NewBadRequestError(message)
    case http.StatusUnauthorized:
        return domain.NewUnauthorizedError(message)
    case http.StatusForbidden:
        return domain.NewForbiddenError(message)
    case http.StatusNotFound:
        return domain.NewNotFoundError(message)
    case http.StatusConflict:
        return domain.NewConflictError(message)
    case http.StatusInternalServerError:
        return domain.NewInternalServerError(message)
    case http.StatusServiceUnavailable:
        return domain.NewServiceUnavailableError(message)
    default:
        return domain.NewInternalServerError("unexpected error")
    }
}
```

### Domain to Goa Error Mapping

**File**: [internal/service/survey_service.go](../internal/service/survey_service.go)

```go
func mapDomainError(err error) error {
    var domainErr *domain.DomainError
    if !errors.As(err, &domainErr) {
        return &survey.InternalServerError{
            Code:    "500",
            Message: "Internal server error",
        }
    }

    switch domainErr.StatusCode {
    case http.StatusBadRequest:
        return &survey.BadRequestError{
            Code:    domainErr.Code,
            Message: domainErr.Message,
        }
    case http.StatusUnauthorized:
        return &survey.UnauthorizedError{
            Code:    domainErr.Code,
            Message: domainErr.Message,
        }
    // ... other cases
    }
}
```

---

## Configuration

### Environment Variables

**Server Configuration**:

```bash
PORT=8080
LOG_LEVEL=info
LOG_ADD_SOURCE=true
```

**Authentication**:

```bash
JWKS_URL=https://heimdall.dev.lfx.linuxfoundation.org/.well-known/jwks.json
AUDIENCE=lfx-v2-survey-service
# For local dev only:
JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL=test-user
```

**ITX Integration** (OAuth2 M2M with Auth0):

```bash
ITX_BASE_URL=https://api.dev.itx.linuxfoundation.org
ITX_AUTH0_DOMAIN=linuxfoundation-dev.us.auth0.com
ITX_CLIENT_ID=<client-id>
ITX_CLIENT_PRIVATE_KEY=<rsa-private-key-pem>
ITX_AUDIENCE=https://api.dev.itx.linuxfoundation.org/
```

**ID Mapping** (NATS):

```bash
NATS_URL=nats://localhost:4222
# For local dev only:
ID_MAPPING_DISABLED=true
```

### Helm Configuration

**File**: [charts/lfx-v2-survey-service/values.yaml](../charts/lfx-v2-survey-service/values.yaml)

```yaml
app:
  environment:
    PORT:
      value: "8080"
    LOG_LEVEL:
      value: info
    ITX_BASE_URL:
      value: https://api.dev.itx.linuxfoundation.org
    ITX_AUTH0_DOMAIN:
      value: linuxfoundation-dev.us.auth0.com
    ITX_AUDIENCE:
      value: https://api.dev.itx.linuxfoundation.org/
    NATS_URL:
      value: nats://lfx-platform-nats.lfx.svc.cluster.local:4222
    JWKS_URL:
      value: https://heimdall.dev.lfx.linuxfoundation.org/.well-known/jwks.json
    AUDIENCE:
      value: lfx-v2-survey-service

  # Secrets loaded from AWS Secrets Manager via External Secrets Operator
  secrets:
    - name: ITX_CLIENT_ID
      path: /cloudops/managed-secrets/auth0/LFX_V2_Surveys_Service
      key: client_id
    - name: ITX_CLIENT_PRIVATE_KEY
      path: /cloudops/managed-secrets/auth0/LFX_V2_Surveys_Service
      key: client_private_key
```

---

## Authorization

### Heimdall RuleSet

**File**: [charts/lfx-v2-survey-service/templates/ruleset.yaml](../charts/lfx-v2-survey-service/templates/ruleset.yaml)

Authorization is handled by Heimdall with OpenFGA checks:

```yaml
- id: "rule:lfx:lfx-v2-survey-service:surveys:create"
  match:
    methods: [POST]
    routes:
      - path: /surveys
  execute:
    - authenticator: oidc
    - authorizer: openfga_check
      config:
        values:
          relation: writer
          object: "project:{{- .Request.Body.project_uid -}}"
    - finalizer: create_jwt
```

**Permission Model**:

- `writer` - Can create, update, delete surveys
- `viewer` - Can read survey details
- `results_viewer` - Can view survey results
- `participant` - Can submit survey responses
- `owner` - Can update their own responses
- `auditor` - Can view response details

---

## Testing Strategy

### Unit Tests

1. **Service Layer Tests** (with mock proxy client)
   - Test authentication parsing
   - Test ID mapping logic
   - Test error handling
   - Test field mapping

2. **Proxy Client Tests** (with mock HTTP server)
   - Test HTTP request construction
   - Test OAuth2 token addition
   - Test error mapping
   - Test response parsing

3. **Converter Tests**
   - Validate field mapping (project_uid ↔ project_id)
   - Validate response conversion

### Integration Tests

1. **End-to-End Flow**
   - Mock Heimdall authentication
   - Mock NATS ID mapping
   - Mock ITX HTTP responses
   - Validate complete request/response flow

### Example Test

```go
func TestScheduleSurvey(t *testing.T) {
    // Setup mocks
    mockProxy := &MockProxyClient{}
    mockIDMapper := &MockIDMapper{}
    mockAuth := &MockAuth{}

    service := NewSurveyService(mockProxy, mockIDMapper, mockAuth, logger)

    // Mock ID mapping: v2 UUID → v1 Salesforce ID
    mockIDMapper.On("MapProjectV2ToV1", mock.Anything, "v2-uuid").
        Return("v1-sfdc-id", nil)

    // Mock ITX response
    mockProxy.On("ScheduleSurvey", mock.Anything, mock.MatchedBy(func(req *itx.SurveyScheduleRequest) bool {
        // Verify field mapping happened
        return req.ProjectID == "v1-sfdc-id" && req.SurveyTitle == "Q1 Survey"
    })).Return(&itx.SurveyScheduleResponse{
        ID:          "survey-123",
        ProjectID:   "v1-sfdc-id",
        SurveyTitle: "Q1 Survey",
    }, nil)

    // Execute
    result, err := service.ScheduleSurvey(ctx, &survey.ScheduleSurveyPayload{
        ProjectUID:  "v2-uuid",
        SurveyTitle: "Q1 Survey",
    })

    // Verify
    assert.NoError(t, err)
    assert.Equal(t, "survey-123", result.ID)
    assert.Equal(t, "Q1 Survey", result.SurveyTitle)
    mockProxy.AssertExpectations(t)
    mockIDMapper.AssertExpectations(t)
}
```

---

## Summary

### Architecture Characteristics

| Characteristic | Implementation |
|---------------|----------------|
| **Type** | Stateless HTTP proxy |
| **Storage** | None (all data in ITX/SurveyMonkey) |
| **Authentication** | JWT (Heimdall) → OAuth2 M2M (Auth0) |
| **Authorization** | OpenFGA via Heimdall |
| **Field Mapping** | Minimal (only project_uid ↔ project_id) |
| **ID Mapping** | V2 UUID ↔ V1 Salesforce ID (via NATS) |
| **Business Logic** | Thin proxy layer |
| **Code Size** | ~2000 LOC |

### Key Design Decisions

1. **Stateless Proxy**: No local persistence simplifies deployment and scaling
2. **Minimal Field Mapping**: Only project identifier differs between APIs
3. **Automatic OAuth2**: Transport layer handles token acquisition and refresh
4. **Domain Error Pattern**: Consistent error handling across all layers
5. **Clean Architecture**: Clear separation between API, service, and infrastructure layers
6. **Goa Framework**: Type-safe API definitions with generated code

### Benefits

- **Simple Integration**: Thin proxy reduces complexity
- **No State Management**: ITX handles all survey lifecycle
- **Centralized SurveyMonkey Access**: ITX manages credentials and API complexity
- **Fast Implementation**: Minimal business logic required
- **Easy Testing**: Mock proxy client for unit tests
- **Scalable**: Stateless design allows horizontal scaling
