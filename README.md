# LFX V2 Survey Service

A proxy service that provides a REST API wrapper around the ITX survey system, built using the Goa framework.

## Overview

The LFX V2 Survey Service acts as a secure intermediary between LFX Platform V2 and the ITX survey backend. It handles authentication, authorization, ID mapping between v1 and v2 systems, and provides a clean REST API interface for survey management.

**Proxy Architecture**: This service is a stateless HTTP proxy that translates LFX v2 REST API calls into ITX API calls. All survey data is stored and managed by the ITX service and SurveyMonkey. The proxy handles:

- **Authentication Translation**: JWT (Heimdall) → OAuth2 M2M (Auth0)
- **Field Mapping**: `project_uid` (v2 UUID) → `project_id` (v1 Salesforce ID)
- **Authorization**: Fine-grained access control via OpenFGA
- **Path Translation**: Shorter proxy paths (`/surveys/{id}`) → ITX paths (`/v2/surveys/{id}/schedule`)

The service also processes real-time NATS events to sync v1 survey data to the v2 indexer and FGA. See [Event Processing Documentation](docs/event-processing.md) for details.

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

### API Documentation

**Survey Service OpenAPI Spec**:

- [Dev](https://lfx-api.dev.v2.cluster.linuxfound.info/_survey/openapi3.yaml)
- [Production](https://lfx-api.v2.cluster.lfx.dev/_survey/openapi3.yaml)

Or import `gen/http/openapi.yaml` into [Swagger Editor](https://editor.swagger.io/) when running locally.

**ITX API Docs** (upstream):

- [Dev](https://api.dev.itx.linuxfoundation.org/explore/?urls.primaryName=v2)
- [Staging](https://api.stg.itx.linuxfoundation.org/explore/?urls.primaryName=v2)
- [Production](https://api.prod.itx.linuxfoundation.org/explore/?urls.primaryName=v2)

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

### Run

Copy `.env.example` to `.env`, fill in the required values (see the `REQUIRED` comments, credentials from 1Password), then source it and run:

```bash
cp .env.example .env
# edit .env and set ITX_CLIENT_ID and ITX_CLIENT_PRIVATE_KEY
source .env

make run
# or for debug logging
make debug
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

## Kubernetes Deployment

### Install with Helm

#### Create Kubernetes Secret

Before installing the chart, you must create a Kubernetes secret with the required credentials. Get the values from the **LFX Platform Chart Values Secrets - Local Development** note in the **LFX V2** vault in 1Password.

```bash
kubectl create secret generic lfx-v2-survey-service -n lfx \
  --from-literal=ITX_CLIENT_ID="<from-1password>" \
  --from-file=ITX_CLIENT_PRIVATE_KEY=/path/to/private.key
```

#### Install from GHCR (no local code changes)

To install the latest published image directly from GHCR without any local modifications:

```bash
make helm-install
```

#### Install with Local Code Changes

Copy the local values example file:

```bash
cp charts/lfx-v2-survey-service/values.local.example.yaml charts/lfx-v2-survey-service/values.local.yaml
```

After making code changes, build the Docker image:

```bash
make docker-build
```

Install the chart using your local image:

```bash
make helm-install-local
```

**Chart Versioning**: The chart version in `Chart.yaml` is set to `0.0.1` and should not be manually incremented. During the release process, the chart version is dynamically set to match the Git tag version (e.g., tag `v0.1.6` results in chart version `0.1.6`).

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
│       └── eventing/         # Event processing handlers
├── gen/                      # Generated code (from Goa)
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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on development setup, code style, how to add new endpoints, commit conventions, and the pull request process.

## License

Copyright The Linux Foundation and each contributor to LFX.

Licensed under the MIT License. See [LICENSE](LICENSE) for details.

