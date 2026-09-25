# Contributing to LFX V2 Survey Service

Contributions are what make the open-source community such an amazing place to learn, inspire, and create.

Thank you for your interest in contributing to the LFX V2 Survey Service! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Local Infrastructure (NATS + Heimdall)](#local-infrastructure-nats--heimdall)
- [Getting Dev Credentials](#getting-dev-credentials)
- [License Headers](#license-headers)
- [Code Style](#code-style)
- [Architecture Guidelines](#architecture-guidelines)
- [Adding New Endpoints](#adding-new-endpoints)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)

## Code of Conduct

By participating in this project, you agree to abide by the [Linux Foundation Code of Conduct](https://www.linuxfoundation.org/code-of-conduct/).

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Create a new branch for your feature or bug fix
4. Make your changes
5. Push your changes to your fork
6. Submit a pull request

## Development Setup

Please refer to the [README.md](README.md) for detailed setup instructions, including all environment variables and how to run the service locally.

### Quick Start

```bash
# Install dependencies (Go 1.25+ required)
make deps

# Generate API code from Goa design
make apigen

# Build the service
make build

# Set up your local environment (fill in ITX credentials — see below)
cp .env.example .env

# Run the service
source .env && make run
```

The [.env.example](.env.example) file has all variables pre-configured for local development with sensible defaults — JWT auth, NATS ID mapping, and event processing are all disabled out of the box. The only values you need to fill in are `ITX_CLIENT_ID` and `ITX_CLIENT_PRIVATE_KEY`.

Run `make help` to see all available targets.

## Local Infrastructure (NATS + Heimdall)

The service depends on NATS and Heimdall. Install the [lfx-platform Helm chart](https://github.com/linuxfoundation/lfx-v2-helm/tree/main/charts/lfx-platform) to run both locally:

```bash
kubectl create namespace lfx

# Latest version (may include breaking changes):
helm install -n lfx lfx-platform \
  oci://ghcr.io/linuxfoundation/lfx-v2-helm/chart/lfx-platform

# Pinned version (recommended for reproducible local setup):
helm install -n lfx lfx-platform \
  oci://ghcr.io/linuxfoundation/lfx-v2-helm/chart/lfx-platform \
  --version <version>
```

For available versions, see the [lfx-v2-helm releases](https://github.com/linuxfoundation/lfx-v2-helm/releases).

This provides NATS, Heimdall, Traefik, OpenFGA, and other platform services. The default `NATS_URL` in [.env.example](.env.example) connects to this chart's NATS instance automatically.

If you want to skip the cluster entirely, the defaults in [.env.example](.env.example) already disable NATS and Heimdall — just fill in the ITX credentials and run.

## Getting Dev Credentials

The service requires ITX OAuth2 credentials (`ITX_CLIENT_ID` and `ITX_CLIENT_PRIVATE_KEY`). Find them in:

> **1Password** → Linux Foundation org → **LFX V2** vault → **LFX Platform Chart Values Secrets - Local Development** (secure note)

- The private key is an RSA key in PEM format
- Store the key locally at `tmp/local.private.key` (gitignored) and reference it with `$(cat tmp/local.private.key)`

For local development, you can bypass NATS and JWT auth entirely using the flags shown in the Quick Start above. The only credential you truly need to make proxy calls to the ITX dev environment is `ITX_CLIENT_ID` and `ITX_CLIENT_PRIVATE_KEY`.

## License Headers

**IMPORTANT**: All source code files must include the appropriate license header. This is enforced by our CI/CD pipeline.

### Required Format

The license header must appear at the top of every source file and must contain:

```text
Copyright The Linux Foundation and each contributor to LFX.
SPDX-License-Identifier: MIT
```

### File Type Examples

#### Go Files (.go)

```go
// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package yourpackage
```

#### YAML Files (.yml, .yaml)

```yaml
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT
```

#### Makefile / Shell Scripts

```bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT
```

### Automated Checks

- **CI Pipeline**: GitHub Actions verifies all files have proper headers on every pull request (excludes `gen/*` and `cmd/survey-api/kodata/*`)

## Code Style

### General Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting — run `make fmt` before committing
- All exported functions and types must have doc comments
- Domain errors use the `DomainError` pattern (see `internal/domain/errors.go`)
- Use structured logging (`slog`) — never `fmt.Println` or `log.Printf`

### Linting

The project uses `golangci-lint`. Run linting before committing:

```bash
# Run linter
make lint

# Check formatting + lint without modifying files
make check
```

[revive.toml](revive.toml) configures the standalone `revive` linter used by MegaLinter in CI. `make lint` runs `golangci-lint` with its built-in defaults (no `.golangci.yml` in this repo).

## Architecture Guidelines

### Respect the Layer Separation

This service uses clean architecture with clear layer boundaries. Understand them before making changes:

- **`api/survey/v1/design/`** — Goa DSL only; no business logic
- **`internal/domain/`** — interfaces and domain models; no external dependencies
- **`internal/service/`** — business logic; depends only on domain interfaces
- **`internal/infrastructure/`** — external integrations (ITX proxy, NATS, auth)
- **`cmd/survey-api/`** — wiring, converters, and Goa interface implementation

Dependencies flow inward. Infrastructure depends on domain, not the other way around.

### Fixing Problems at the Source

When something doesn't work, fix the root cause. Disabling auth, bypassing linting with `//nolint` without explanation, or adding build hacks are not fixes. `TODO: TEMPORARY` bypasses have a tendency to reach production.

### Architectural Changes Need Their Own PRs

Changes that affect the entire application — middleware, NATS consumer configuration, new infrastructure integrations — are architectural decisions. They need standalone PRs with focused review, not bundled inside feature work.

## Adding New Endpoints

This service is a proxy. Every endpoint here corresponds to an endpoint in the upstream ITX SurveyMonkey service — **the ITX endpoint must exist before you proxy it here**. If the capability you need doesn't exist in ITX yet, start there: [github.com/linuxfoundation-it/itx-service-survey-monkey](https://github.com/linuxfoundation-it/itx-service-survey-monkey).

Once the ITX endpoint is available, follow these steps in order to proxy it:

### 1. Define the endpoint in the Goa DSL

Edit `api/survey/v1/design/survey.go` and add a new `Method(...)` block inside the `Service("survey", ...)` declaration. Add any new request/response types to `api/survey/v1/design/types.go`.

```go
Method("my_new_endpoint", func() {
    Description("Description of what this proxies to in ITX")
    Payload(func() {
        BearerTokenAttribute()
        Attribute("survey_uid", String, "Survey UID")
        Required("survey_uid")
    })
    Result(MyNewResult)
    HTTP(func() {
        POST("/surveys/{survey_uid}/my-action")
        Response(StatusOK)
    })
})
```

### 2. Regenerate the Goa code

```bash
make apigen
```

This overwrites `gen/` with updated handlers, types, and OpenAPI spec. Do not manually edit files in `gen/`.

### 3. Update the domain proxy interface

Add the new method signature to the appropriate interface in `internal/domain/proxy.go` (`SurveyClient` or `SurveyResponseClient`).

### 4. Implement the ITX proxy call

Add the corresponding method to `internal/infrastructure/proxy/client.go`. This is where the HTTP call to ITX is made, including the path translation (e.g., `/surveys/{id}` → `/v2/surveys/{id}/schedule`). Add any new ITX request/response models to `pkg/models/itx/`.

### 5. Add the service method

Implement the business logic in `internal/service/survey_service.go`. This is where authentication parsing, ID mapping (v2 UUID → v1 Salesforce ID), field transformation, and error mapping happen. See the existing methods for the standard pattern.

### 6. Implement the API handler

Add the method to `cmd/survey-api/api.go`, which implements the Goa-generated interface. This should be a thin delegating call to the service layer.

### 7. Add tests

Write unit tests for the service method using mock implementations of the domain interfaces.

### Checklist

- [ ] ITX upstream endpoint exists in [itx-service-survey-monkey](https://github.com/linuxfoundation-it/itx-service-survey-monkey)
- [ ] Goa DSL updated and `make apigen` run — generated files committed
- [ ] Domain proxy interface updated
- [ ] ITX proxy method implemented with correct path mapping
- [ ] Service method implemented with ID mapping and error handling
- [ ] API handler added
- [ ] Unit tests written
- [ ] License headers on all new files
- [ ] `make check` and `make test` pass

## Commit Messages

### Format

Follow the conventional commit format:

```text
type(scope): subject

body

footer
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

### Examples

```text
feat(surveys): add bulk resend endpoint

Implements POST /surveys/{survey_uid}/bulk_resend to proxy the
ITX bulk email resend call.

Closes #42
```

### Sign-off

All commits must be signed off per the [Developer Certificate of Origin](https://developercertificate.org/):

```bash
git commit --signoff
```

This adds a `Signed-off-by` line to your commit message.

## Pull Request Process

### Branch Naming

Use the format `<type>/<ticket-or-description>`, for example:

- `feat/LFXV2-123-add-bulk-resend`
- `fix/LFXV2-456-fix-id-mapping`
- `chore/update-dependencies`

### PR Scope

- **Keep PRs focused on a single concern** — a feature PR should contain only the feature
- **Architectural decisions require their own PR** — changes to middleware, infrastructure wiring, or NATS configuration need standalone discussion and approval
- **Never mix security changes with feature work** — auth middleware modifications must be reviewed independently

### PR Checklist

1. **Update Documentation**: Update README or `docs/` for any new features or configuration changes
2. **Add Tests**: Include unit tests for new functionality
3. **Pass All Checks**: Ensure `make check` and `make test` pass locally
4. **License Headers**: Verify all new files have the correct license header
5. **API Generation**: If you modified Goa design files, run `make apigen` and commit the regenerated files
6. **Clear Description**: Provide a clear description of changes and link any related issues or Jira tickets

### PR Title Format

Use the same conventional commit format for PR titles:

```text
feat(surveys): add bulk resend endpoint
```

## Testing

### Running Tests

```bash
# Run all tests
make test
```

### Test Requirements

- All new features must include unit tests
- Tests must pass with `-race` flag (already enforced by `make test`)
- Mock external dependencies using the domain interfaces — tests should not require a live ITX or NATS connection
