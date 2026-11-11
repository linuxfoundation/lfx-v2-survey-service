# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

```bash
make deps         # Install goa CLI and golangci-lint
make apigen       # Generate API code from Goa DSL (run after changing api/survey/v1/design/)
make build        # Compile binary to bin/survey-api
make run          # Run service on port 8080 (requires .env)
make debug        # Run with LOG_LEVEL=debug
make test         # Run tests with race detector (5m timeout)
make lint         # Run golangci-lint
make fmt          # Format code with gofmt
make check        # Check format & lint without modifying
make verify       # Verify generated code is up to date
make docker-build # Build Docker image
```

**Single test**: `go test -run TestFunctionName ./path/to/package/...`

**Local dev setup**:
```bash
cp .env.example .env
# Fill in ITX_CLIENT_ID and ITX_CLIENT_PRIVATE_KEY from 1Password
source .env
make run
```

**Required env vars** (no defaults): `ITX_CLIENT_ID`, `ITX_CLIENT_PRIVATE_KEY` (RSA PEM)

## Architecture

This service is a **stateless HTTP proxy** between LFX Platform V2 clients and the ITX (Linux Foundation IT) survey backend ([itx-service-survey-monkey](https://github.com/linuxfoundation-it/itx-service-survey-monkey)). Its core job is:

1. **Auth translation**: Accepts Heimdall-issued JWTs (PS256) from v2 clients; issues OAuth2 M2M tokens (RSA private key) to ITX
2. **ID translation**: v2 UUIDs ↔ v1 Salesforce IDs via NATS request/reply (`lfx.lookup_v1_mapping`)
3. **Event processing**: Consumes NATS JetStream KV bucket changes to sync v1 survey data into the v2 indexer

### Layered Structure

```
api/survey/v1/design/   ← Goa DSL (source of truth for API shape)
gen/                    ← Generated Goa code (DO NOT edit manually)
cmd/survey-api/
  main.go               ← Bootstrap, config, graceful shutdown
  api.go                ← Goa adapter (implements generated interfaces)
  eventing/             ← NATS event processing (v1→v2 transformation)
internal/
  domain/               ← Interfaces + error types (no implementations)
  service/              ← Business logic orchestration
  infrastructure/
    auth/               ← JWT validation (Heimdall JWKS)
    proxy/              ← ITX HTTP client (OAuth2 M2M)
    idmapper/           ← NATS-based ID mapper (+ noop for testing)
    eventing/           ← NATS event publisher
  middleware/           ← RequestID, logging, OpenFGA authorization
pkg/                    ← Shared utilities and ITX API models
```

### Key Patterns

**Goa DSL flow**: Edit `api/survey/v1/design/survey.go` → run `make apigen` → implement new methods in `cmd/survey-api/api.go` and `internal/service/survey_service.go`. Generated code in `gen/` is overwritten on each `apigen` run. See [CONTRIBUTING.md](CONTRIBUTING.md#adding-new-endpoints) for the full step-by-step checklist.

**Domain interfaces**: All infrastructure dependencies (auth, proxy, ID mapper, event publisher) are defined as interfaces in `internal/domain/` and injected at startup in `main.go`. Use `noop_mapper.go` pattern for test doubles.

**Error handling**: Use `domain.DomainError` with typed severities (`ValidationError`, `NotFound`, `Conflict`, `Internal`, `Unavailable`). The Goa layer maps these to HTTP status codes.

**ID mapping**: A v2 `committee_uid` maps to a v1 compound ID `{project_sfid}:{committee_sfid}`. The mapper first resolves the project UID to a Salesforce ID via NATS, then constructs the compound format. Set `ID_MAPPING_DISABLED=true` to skip mapping (uses noop mapper).

**Event processing**: The `eventing/event_processor.go` watches NATS JetStream KV buckets for keys matching `$KV.v1-objects.itx-surveys.>` and `$KV.v1-objects.itx-survey-responses.>`. Handlers transform v1 payloads to v2 `SurveyData`/`SurveyResponseData` and publish to the v2 indexer via NATS. Durable consumer: `survey-service-kv-consumer`, max 3 redeliveries.

**Creator identity**: `ScheduleSurvey` and `UpdateSurvey` never trust `creator_*` values from the request body. The creator is the JWT principal (LFID username); the display name comes from the auth service (`lfx.auth-service.user_metadata.read`) through the optional `UserNameReader`, falling back to the username. ITX stores the name as sent and does not look it up.

**Test file naming**: a `_test.go` file must match an implementation file in the same directory (`foo_test.go` tests `foo.go`). Never add themed or suffixed test files such as `foo_gaps_test.go` or `foo_coverage_test.go`; put tests in the `_test.go` of the file that defines the code under test. Only shared fakes/helpers (`mock_*_test.go`, `helpers_test.go`) may lack a counterpart. Find violations with: `find . -name '*_test.go' ! -path './gen/*' | while read f; do [ -f "${f%_test.go}.go" ] || echo "$f"; done`.

**Test doubles and harnesses**: reuse these instead of writing new fakes.
- `internal/domain/mocks` has function-field mocks for every domain interface (service tests); an unconfigured method panics, so a test fails loudly if it calls something it did not set up.
- `internal/infrastructure/proxy/client_test.go` has an `operation` table harness (`runOperations`). A new ITX proxy method gets one `operation` row in the `_test.go` of the file that defines it (`survey_proxy_test.go`, `response_proxy_test.go`, `exclusion_proxy_test.go`, `client_test.go`); the harness checks method, path, query, body, `x-scope` header, decoded result and 404 mapping.
- `cmd/survey-api/eventing/helpers_test.go` has `fakeKV` (a stateful in-memory `jetstream.KeyValue` for Get/Put), `newRecordingPublisher` and `discardLogger` for the KV event handlers. Assert on what was published and stored rather than on call sequences. For code that needs a real connection (`NewEventProcessor`, the invite subscriber), `startTestNATS` runs an in-process JetStream server on a random port.
- Give every field in a "maps every field" fixture a distinct value, so a swapped pair of fields cannot pass.

**Structured logging**: Uses `slog` throughout. The `internal/logging/` package sets up OpenTelemetry-aware structured logging. Pass the logger via context or direct injection — avoid `log.Print*`.
