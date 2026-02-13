# LFX V2 Survey Service

A proxy service that provides a REST API wrapper around the ITX survey system, built using the Goa framework.

## Overview

The LFX V2 Survey Service acts as a secure intermediary between LFX Platform V2 and the ITX survey backend. It handles authentication, authorization, ID mapping between v1 and v2 systems, and provides a clean REST API interface for survey management.

**Proxy Architecture**: This service is a stateless HTTP proxy that translates LFX v2 REST API calls into ITX API calls. All survey data is stored and managed by the ITX service and SurveyMonkey. The proxy handles:

- **Authentication Translation**: JWT (Heimdall) → OAuth2 M2M (Auth0)
- **Field Mapping**: `project_uid` (v2 UUID) → `project_id` (v1 Salesforce ID)
- **Authorization**: Fine-grained access control via OpenFGA
- **Path Translation**: Shorter proxy paths (`/surveys/{id}`) → ITX paths (`/v2/surveys/{id}/schedule`)

See [ITX Proxy Implementation Architecture](docs/itx-proxy-implementation.md) for detailed information.

## Features

- **Survey Scheduling**: Schedule surveys to be sent to committee members
- **JWT Authentication**: Secure authentication via Heimdall JWT tokens
- **OAuth2 M2M**: Machine-to-machine authentication with ITX using Auth0
- **ID Mapping**: Automatic v1/v2 ID translation via NATS
- **Event Processing**: Real-time sync of v1 survey data to v2 indexer and FGA (see [Event Processing](docs/event-processing.md))
- **OpenFGA Authorization**: Fine-grained access control
- **OpenAPI Spec**: Auto-generated from Goa design
- **Kubernetes Ready**: Includes Helm charts with health checks and probes

## Architecture

```
┌──────────────┐     JWT      ┌──────────────────┐    OAuth2    ┌──────────┐
│   LFX V2     │─────────────▶│  Survey Service  │─────────────▶│   ITX    │
│   Clients    │              │  (This Service)  │              │  API     │
└──────────────┘              └──────────────────┘              └──────────┘
                                       │
                                       │ NATS
                                       ▼
                                  ┌─────────┐
                                  │   ID    │
                                  │ Mapper  │
                                  └─────────┘
```

## API Endpoints

The service provides 15 REST API endpoints for survey management:

### Survey Management

- `POST /surveys` - Create and schedule a new survey for a single committee
- `GET /surveys/{survey_uid}` - Get survey details
- `PUT /surveys/{survey_uid}` - Update survey (when status is 'disabled')
- `DELETE /surveys/{survey_uid}` - Delete survey (when status is 'disabled')
- `POST /surveys/{survey_uid}/bulk_resend` - Bulk resend survey emails to select recipients
- `GET /surveys/{survey_uid}/preview_send` - Preview recipients affected by a resend
- `POST /surveys/{survey_uid}/send_missing_recipients` - Send survey to committee members who haven't received it
- `DELETE /surveys/{survey_uid}/recipient_group` - Remove a recipient group from survey

### Survey Responses

- `DELETE /surveys/{survey_uid}/responses/{response_id}` - Delete survey response
- `POST /surveys/{survey_uid}/responses/{response_id}/resend` - Resend survey email to specific user

### Exclusions Management

- `POST /surveys/exclusion` - Create survey or global exclusion
- `DELETE /surveys/exclusion` - Delete survey or global exclusion
- `GET /surveys/exclusion/{exclusion_id}` - Get exclusion by ID
- `DELETE /surveys/exclusion/{exclusion_id}` - Delete exclusion by ID

### Utilities

- `POST /surveys/validate_email` - Validate email template body and subject

See the OpenAPI spec at `/openapi.yaml` or `/openapi.json` when running locally.

### API Documentation

For detailed API contracts showing request/response schemas and differences between the proxy API and ITX API:

- [Survey Management API Contracts](docs/api-contracts/itx-surveys-api.md)
- [Survey Responses API Contracts](docs/api-contracts/itx-survey-responses-api.md)
- [Exclusions API Contracts](docs/api-contracts/itx-exclusions-api.md)
- [ITX Proxy Implementation Architecture](docs/itx-proxy-implementation.md)

## Prerequisites

- Go 1.25.4 or later
- Docker (for containerization)
- Kubernetes cluster (for deployment)
- Helm 3 (for deployment)
- Access to ITX API
- Auth0 M2M credentials with RSA private key
- NATS server (for ID mapping)

## Development Setup

### Install Dependencies

```bash
make deps
```

This installs:

- `goa` CLI for code generation
- `golangci-lint` for linting

### Generate Code

```bash
make apigen
```

This generates server/client code from the Goa design in `api/survey/v1/design/`.

### Build

```bash
make build
```

Binary is output to `bin/survey-api`.

### Run Locally

```bash
# Set required environment variables
export ITX_CLIENT_ID="your-client-id"
export ITX_CLIENT_PRIVATE_KEY="$(cat path/to/private-key.pem)"
export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-user"  # For local dev only
export ID_MAPPING_DISABLED="true"  # For local dev without NATS

# Run the service
make run
```

The service will start on port 8080 by default.

### Testing

```bash
make test
```

### Linting and Formatting

```bash
make lint
make fmt
```

## Configuration

The service is configured via environment variables:

### Server Configuration

- `PORT` - HTTP server port (default: 8080)
- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)
- `LOG_ADD_SOURCE` - Add source file info to logs (default: true)

### Authentication

- `JWKS_URL` - Heimdall JWKS endpoint for JWT validation
- `AUDIENCE` - Expected JWT audience (default: lfx-v2-survey-service)
- `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` - Mock principal for local dev (disables JWT validation)

### ITX Integration

- `ITX_BASE_URL` - ITX API base URL
- `ITX_AUTH0_DOMAIN` - Auth0 domain for M2M authentication
- `ITX_CLIENT_ID` - Auth0 client ID
- `ITX_CLIENT_PRIVATE_KEY` - RSA private key in PEM format (not base64-encoded)
- `ITX_AUDIENCE` - Auth0 API audience

### ID Mapping

- `NATS_URL` - NATS server URL for ID mapping
- `ID_MAPPING_DISABLED` - Disable ID mapping for local dev (default: false)

### Event Processing

- `EVENT_PROCESSING_ENABLED` - Enable/disable event processing (default: true)
- `EVENT_CONSUMER_NAME` - JetStream consumer name (default: survey-service-kv-consumer)
- `EVENT_STREAM_NAME` - JetStream stream name (default: KV_v1-objects)
- `EVENT_FILTER_SUBJECT` - NATS subject filter (default: $KV.v1-objects.>)

See [Event Processing Documentation](docs/event-processing.md) for details.

## Docker

### Build Image

```bash
make docker-build
```

### Run Container

```bash
docker run -p 8080:8080 \
  -e ITX_CLIENT_ID="your-client-id" \
  -e ITX_CLIENT_PRIVATE_KEY="$(cat private-key.pem)" \
  linuxfoundation/lfx-v2-survey-service:latest
```

## Kubernetes Deployment

### Install with Helm

```bash
# Install with default values
make helm-install

# Install with local values file
make helm-install-local
```

### Configure Secrets

The service requires Auth0 credentials stored in AWS Secrets Manager:

1. Create a secret in AWS Secrets Manager at path:
   `/cloudops/managed-secrets/auth0/LFX_V2_Surveys_Service`

2. Add the following keys:
   - `client_id` - Auth0 client ID
   - `client_private_key` - RSA private key in raw PEM format (not base64)

3. The External Secrets Operator will sync these to Kubernetes automatically

### Verify Deployment

```bash
kubectl get pods -n lfx -l app=lfx-v2-survey-service
kubectl logs -n lfx -l app=lfx-v2-survey-service
```

## Project Structure

```
.
├── api/                      # Goa design files
│   └── survey/v1/design/     # API design (DSL)
├── cmd/                      # Application entry points
│   └── survey-api/           # Main service binary
├── gen/                      # Generated code (from Goa)
├── cmd/                      # Application entry points
│   └── survey-api/           # Main service binary
│       └── eventing/         # Event processing handlers
├── internal/                 # Private application code
│   ├── domain/               # Domain interfaces and types
│   ├── infrastructure/       # Infrastructure implementations
│   │   ├── auth/             # JWT authentication
│   │   ├── eventing/         # Event processing infrastructure
│   │   ├── idmapper/         # ID mapping (NATS)
│   │   └── proxy/            # ITX proxy client
│   ├── logging/              # Structured logging
│   ├── middleware/           # HTTP middleware
│   └── service/              # Business logic
├── pkg/                      # Public packages
│   ├── constants/            # Shared constants
│   └── models/itx/           # ITX API models
├── docs/                     # Documentation
│   ├── api-contracts/        # API contract documentation
│   ├── event-processing.md   # Event processing guide
│   └── itx-proxy-implementation.md  # Architecture guide
├── charts/                   # Helm charts
│   └── lfx-v2-survey-service/
├── Dockerfile                # Container image
├── Makefile                  # Build automation
└── go.mod                    # Go module definition
```

## Development Workflow

1. **Design API**: Modify `api/survey/v1/design/*.go`
2. **Generate Code**: Run `make apigen`
3. **Implement Logic**: Add/update files in `internal/service/`
4. **Test**: Run `make test`
5. **Format & Lint**: Run `make fmt lint`
6. **Build**: Run `make build`
7. **Verify**: Run `make verify` to ensure generated code is up to date

## Contributing

### Code Style

- Follow standard Go conventions
- Use `gofmt` and `golangci-lint`
- Write tests for business logic
- Update API design in DSL (don't manually edit generated code)

### Commit Guidelines

When committing changes, follow the repository's commit conventions and include:

```
Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

## License

Copyright The Linux Foundation and each contributor to LFX.

Licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Support

For issues, questions, or contributions, please open an issue in the GitHub repository.
