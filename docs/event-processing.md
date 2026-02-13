# Event Processing

## Overview

The survey service implements NATS KV bucket event processing to automatically sync survey and survey response data from the v1 system to the v2 system. This enables real-time data synchronization, search indexing, and access control updates without manual intervention.

## Architecture

### Components

```
┌─────────────────┐
│  v1 DynamoDB    │
│   (via KV)      │
└────────┬────────┘
         │
         ├─ itx-surveys:*
         └─ itx-survey-responses:*
         │
         v
┌─────────────────────────────────┐
│   NATS KV Bucket: v1-objects    │
└────────┬────────────────────────┘
         │
         v
┌─────────────────────────────────┐
│     Event Processor             │
│  (JetStream Consumer)           │
└────────┬────────────────────────┘
         │
         ├─ Transform v1 → v2
         ├─ Map IDs (v1 SFID → v2 UUID)
         │
         v
┌────────────────┬────────────────┐
│                │                │
│   Indexer      │   FGA-Sync     │
│   Service      │   Service      │
│                │                │
│ (Search Index) │ (Access Control)
└────────────────┴────────────────┘
```

### Event Flow

1. **Watch**: Event processor watches the `v1-objects` KV bucket for keys matching:
   - `itx-surveys:*` - Survey data
   - `itx-survey-responses:*` - Survey response data

2. **Transform**: Converts v1 format to v2 format:
   - String fields → proper types (strings to ints)
   - v1 SFIDs → v2 UUIDs (committees, projects)
   - Preserves all data including SurveyMonkey answers

3. **Publish**: Sends transformed data to two downstream services:
   - **Indexer Service** (`lfx.index.survey`, `lfx.index.survey_response`)
     - Enables search functionality
     - Includes parent references (committee, project)
     - Provides access control metadata

   - **FGA-Sync Service** (`lfx.fga-sync.update_access`, `lfx.fga-sync.delete_access`)
     - Updates Fine-Grained Authorization (FGA) tuples
     - Manages viewer/auditor permissions
     - Links surveys to committees and projects

4. **Track**: Records processed events in `v1-mappings` KV bucket for deduplication

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EVENT_PROCESSING_ENABLED` | `true` | Enable/disable event processing |
| `EVENT_CONSUMER_NAME` | `survey-service-kv-consumer` | JetStream consumer name |
| `EVENT_STREAM_NAME` | `KV_v1-objects` | JetStream stream name |
| `EVENT_FILTER_SUBJECT` | `$KV.v1-objects.>` | NATS subject filter pattern |
| `NATS_URL` | `nats://nats:4222` | NATS server URL |

### Consumer Configuration

- **Delivery Policy**: `DeliverLastPerSubjectPolicy` - Processes the latest version of each key
- **Ack Policy**: `AckExplicitPolicy` - Requires explicit acknowledgment
- **Max Deliver**: `3` - Retries transient failures up to 3 times
- **Ack Wait**: `30s` - Timeout before message redelivery
- **Max Ack Pending**: `1000` - Maximum unacknowledged messages

## Data Transformation

### Survey Data

**v1 Format (DynamoDB/KV)**:
- All numeric fields stored as strings (e.g., `"nps_value": "8"`)
- v1 SFIDs for committees and projects
- Committee array with per-committee statistics

**v2 Format (Transformed)**:
- Proper types (integers, booleans)
- v2 UUIDs for committees and projects
- Mapped via IDMapper service
- Preserved committee array structure

**Example Transformation**:
```json
// v1 Input
{
  "id": "survey-123",
  "nps_value": "8",
  "total_responses": "42",
  "committees": [{
    "committee_id": "a094V00000A1BcdQAF",  // v1 SFID
    "project_id": "a094V00000A1XyzQAF",     // v1 SFID
    "nps_value": "9"
  }]
}

// v2 Output
{
  "uid": "survey-123",
  "nps_value": 8,                           // Integer
  "total_responses": 42,                    // Integer
  "committees": [{
    "committee_uid": "550e8400-e29b-41d4-a716-446655440000",  // v2 UUID
    "project_uid": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",    // v2 UUID
    "nps_value": 9
  }]
}
```

### Survey Response Data

**v1 Format**: Similar string-based fields, v1 references

**v2 Format**:
- Proper types
- Mapped project/committee/survey UIDs
- Preserved SurveyMonkey question answers (no transformation)

## Error Handling

### Transient Errors (Retry)
These errors trigger NAK (negative acknowledgment) for automatic retry:
- NATS connection timeouts
- IDMapper service unavailable
- Network failures
- Temporary downstream service outages

**Action**: Message redelivered up to `MaxDeliver` times (3 attempts)

### Permanent Errors (Skip)
These errors trigger ACK to skip and move on:
- Invalid JSON structure
- Missing required fields (e.g., empty `id`)
- No parent references (survey/response orphaned)
- Malformed data

**Action**: Log warning and continue processing other messages

### ID Mapping Failures
When v1→v2 ID mapping fails:
- Log warning with v1 ID
- Skip setting v2 UID for that reference
- Continue processing with remaining valid data
- Survey/response may be skipped if critical references missing

## Operations

### Starting the Service

Event processing starts automatically when the service starts:

```bash
# Default (event processing enabled)
./survey-api

# Explicitly enable
EVENT_PROCESSING_ENABLED=true ./survey-api

# Disable event processing
EVENT_PROCESSING_ENABLED=false ./survey-api
```

### Monitoring

**Log Messages**:
```
INFO  Event processing is ENABLED - initializing event processor
INFO  Event processor started in background
INFO  processing survey update key=itx-surveys:survey-123
INFO  successfully sent survey indexer and access messages survey_id=survey-123
```

**Consumer Status**:
```bash
# Check consumer status
nats consumer info KV_v1-objects survey-service-kv-consumer
```

### Lifecycle

1. **Startup**: Event processor initializes after IDMapper
2. **Running**: Processes events in background goroutine
3. **Shutdown**: Graceful shutdown sequence:
   - Context cancellation stops the consumer
   - Consumer drains pending messages
   - NATS connection closed
   - HTTP server shutdown

### Troubleshooting

**No events processing**:
- Check `EVENT_PROCESSING_ENABLED=true`
- Verify NATS connection: `NATS_URL`
- Check consumer exists: `nats consumer ls KV_v1-objects`

**Events failing repeatedly**:
- Check logs for permanent errors
- Verify IDMapper service is running
- Confirm indexer and FGA-sync services are available

**Duplicate processing**:
- Check `v1-mappings` KV bucket for tracking entries
- Verify consumer name is unique per instance

**ID mapping failures**:
- Ensure IDMapper service has v1↔v2 mappings populated
- Check project/committee references exist in v1 system

## Deduplication

The service uses the `v1-mappings` KV bucket to track processed events:

**Key Pattern**:
- Surveys: `survey:{uid}`
- Responses: `survey_response:{uid}`

**Value**: Timestamp of last processing

**Logic**:
- If mapping exists → **UPDATE** operation
- If mapping missing → **CREATE** operation
- After processing → Store/update mapping entry

This ensures:
- First event creates the resource
- Subsequent events update the resource
- No duplicate resources in downstream services

## Performance Considerations

**Concurrency**:
- Single consumer per service instance
- Messages processed sequentially per consumer
- Multiple service instances = parallel processing

**Throughput**:
- `MaxAckPending=1000` allows up to 1000 in-flight messages
- Adjust based on processing speed and resource availability

**Backpressure**:
- Consumer automatically pauses when `MaxAckPending` reached
- Resumes when pending count drops

**Resource Usage**:
- Event processor runs in background goroutine (low overhead)
- NATS connection shared with IDMapper
- Memory footprint minimal (streaming model)

## Related Services

### IDMapper Service
- Maps v1 SFIDs ↔ v2 UUIDs
- Required for event processing
- Queries via NATS request-reply pattern

### Indexer Service
- Receives transformed survey/response data
- Indexes in OpenSearch for search functionality
- Handles `ActionCreated`, `ActionUpdated`, `ActionDeleted`

### FGA-Sync Service
- Receives access control updates
- Manages OpenFGA authorization tuples
- Links resources to parent entities (committees, projects)

## Development

### Testing Event Processing

1. **Disable in local development**:
   ```bash
   export EVENT_PROCESSING_ENABLED=false
   ```

2. **Watch consumer activity**:
   ```bash
   nats consumer next KV_v1-objects survey-service-kv-consumer --count 10
   ```

3. **Trigger test event**:
   ```bash
   # Put test survey in v1-objects KV
   nats kv put v1-objects itx-surveys:test-123 '{"id":"test-123",...}'
   ```

4. **Check processing logs**:
   ```bash
   # Look for processing messages
   grep "processing survey update" logs/survey-api.log
   ```

### Code Structure

```
cmd/survey-api/eventing/
├── event_processor.go           # Lifecycle management
├── kv_handler.go                # Event routing by key prefix
├── survey_event_handler.go      # Survey transformation logic
└── survey_response_event_handler.go  # Response transformation logic

internal/domain/
├── event_models.go              # v2 data models
└── event_publisher.go           # Publisher interface

internal/infrastructure/eventing/
├── event_config.go              # Configuration structs
└── nats_publisher.go            # NATS publishing implementation
```

### Adding New Event Types

To add processing for new entity types:

1. Create handler in `cmd/survey-api/eventing/{entity}_event_handler.go`
2. Add routing logic in `kv_handler.go`
3. Define v2 model in `internal/domain/event_models.go`
4. Add publisher method in `internal/infrastructure/eventing/nats_publisher.go`
5. Update documentation

## References

- [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
- [NATS KV Store](https://docs.nats.io/nats-concepts/jetstream/key-value-store)
- [OpenFGA Authorization](https://openfga.dev/)
- [Voting Service PR #8](https://github.com/linuxfoundation/lfx-v2-voting-service/pull/8) - Reference implementation
