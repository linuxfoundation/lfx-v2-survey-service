# Survey-owned Postgres event state

ITX remains the business-data source of truth and object IDs stay unchanged.
Canonical project/committee translations still use the helper RPC. Only owned
event processing state moves out of the shared `v1-mappings` bucket:

- `survey_processed_resources`: survey/response/template source IDs with empty,
  processed, deleted or legacy-ID state. Application tombstones are retained.
- `survey_invitation_guards`: source response ID, pending/sent/deleted/empty
  status and returned invite ID. No recipient data or invite payloads are stored.
- `survey_invitation_intents`: newly processed responses eligible for an invite,
  recorded before processed state; historical imports do not acquire intent.
  Terminal no-invite decisions set `resolved=true` without removing the row, so
  a later email edit or replay cannot recreate the same eligibility.
- `survey_state_cutover`: offline verification checkpoint, required by runtime.

These are service-owned tables, not the helper's `v1_mappings` table. The adapter
reconstructs legacy entries at the handler boundary but persists named domain
fields. Source `v1-objects` replication and business entities are not migrated.

## Configuration and offline cutover

Set `SURVEY_STATE_DATABASE_URL` from an approved Secret with TLS verification for
deployed databases. Helm `eventStateDatabase.secretName`/`secretKey` inject an
existing single-key DSN Secret. Alternatively configure
`app.environment.SURVEY_STATE_DATABASE_URL.valueFrom`, never both sources.
Database operations are capped at five seconds; pools have at most ten
connections per process. Schema application is an explicit locked transaction,
not an empty-state bootstrap or fallback during runtime startup.

1. Provision a survey-owned database/schema and Secret. A migration role owns
   DDL/checkpoint writes; runtime needs table DML, sequence usage and checkpoint
   reads only. Size capacity for all replicas.
2. Stop/drain all legacy mappings writers and survey consumers/repair tools.
   Keep the approved writer fence through verification and deployment. Keep
   source replication and its durable stream; verify retention headroom.
3. Run `go run ./scripts/migrate_event_state` with `NATS_URL` and the Secret-loaded
   DSN. Dry-run scans and validates owned state without applying schema or writing.
    Scans use leader-served `STREAM.MSG.GET` next-by-subject requests, never
    follower-served direct reads or a bulk ephemeral consumer. Memory
   retains only owned current entries; size the job from measured state volume.
4. Reconcile source/history gaps and deliberate differences from historical
   wrapper copies in the helper's Postgres table. Those copies are not implicitly
   authoritative and are not overwritten or deleted by this tool.
5. Run the same command with `-apply`. Create-only import preserves agreeing
   partial rows, compares every entry semantically and rejects conflicting or
   unmatched target rows. Native DEL/PURGE removes source entries; `!del` is
   retained. Source first/last sequence and message count must remain unchanged.
   Empty/unusable source snapshots abort; unsuccessful imports cannot enable
   runtime. Failure logs have fixed safe categories, never raw driver/source data.
6. Deploy the upgraded consumer with the verified PG Secret while fenced; restore
   other still-legacy writers only afterward. Verify dev first, then production:
   surveys, templates, responses/parent dependencies, delete/replay/restore,
   indexing, FGA events and invitation idempotency. These are operational rollout
   steps; the PR does not execute them.

The importer changes no NATS data or deployed settings and refuses reimport after
a checkpoint exists. `already_verified` means do not erase a checkpoint to force
a retry. `target_conflict`, `target_extra_rows`, `invalid_source_state`,
`empty_source` or `source_changed` require investigation/reconciliation;
`import_busy`/`infrastructure_failure` require checking availability. Verify
checkpoint state before retrying after an ambiguous result.

## Concurrency and rollback

PG revisions come from a shared sequence and remain monotonic across
delete/recreate cycles. Create-only invitation claims select one dispatcher;
completion uses the claimed PG revision. Ambiguous send or completion failures
retain pending state for manual reconciliation, never automatic expiry/resend.
Storage failures retry events rather than being mistaken for absent/created
resources or acknowledged after losing deletion state.

Pre-dispatch guard read/claim failures NAK the event. Its durable invitation intent
lets an unclaimed invitation resume on redelivery even when the response is now
classified as updated. Historical imported response markers without an intent
never become eligible merely because they are replayed or edited. Existing
pending/sent guards always suppress another dispatch.

Recoverable failures have unlimited durable deliveries with capped 2s/10s/20s
backoff and warning diagnostics. Monitor backlog and retained-source headroom;
stream expiry/purges still remove recovery input. Invalid/permanent payloads are
ACKed rather than retried indefinitely.

Handlers serialize the same source key across replicas with PG session advisory
locks. A busy lock retries instead of waiting while holding an event callback.
Leader-served current-source reads compare the delivered stream revision before
publication/dispatch and every PG write; old deletes cannot replay over a newer
restore. NATS publications are flushed before releasing the lock. Current-source
reads need `STREAM.MSG.GET` permission on the configured source stream in addition
to the existing consumer/KV permissions.

This is not a distributed transaction or strict downstream revision fence: a
source change or lost lock connection can race an already-started external write,
and index/FGA messages do not carry enforced source versions. Separate subjects
and downstream consumers may reorder their effects. Strict end-to-end ordering
requires companion downstream version enforcement/outbox work; this PR makes no
such guarantee. The current source's subsequent delivery reconciles handler state.

Pending dispatch claims are never automatically deleted, expired or resent,
including when a source change prevents dispatch after claim creation. Inventory
with a read-only query (no recipient data):

```sql
SELECT response_id, version, updated_at
FROM survey_invitation_guards WHERE state = 'pending'
ORDER BY updated_at, response_id;
```

Reconcile each claim against invite-service evidence and the current source.
If dispatch occurred or is ambiguous, retain the guard; confirmed dispatch can
be completed with the invite ID using the observed revision. Reset only with
explicit operator approval and proof no dispatch occurred, while survey
consumers are fenced. Do not bulk purge guards or use elapsed age as proof.

Before upgraded PG writes, rollback can resume the old image against unchanged
KV. After PG writes, KV is stale: fence/drain processing and reconcile an approved
rollback target before using the old image. There is no dual-write or stale-read
fallback. Remove obsolete shared KV/PG copies only after verified owner cutovers
and explicit cleanup approval; bucket retirement is still gated by other services
and read-only tooling in ops #1649.

## Tests

Set `SURVEY_STATE_TEST_DATABASE_URL` to a disposable local Postgres database and
run `make test`. Tests create/drop only unique test schemas. Without the variable,
database tests skip. Real PG tests cover state distinctions, CAS, delete/recreate
revision safety and concurrent invitation claims. Migration tests use a real
in-process NATS server to exercise all four owned families, interleaved unrelated
subjects, multiple revisions, native deletes/purges/recreation and target conflicts.
Existing stateful handler tests cover retries and output behavior. CI supplies a
disposable PG service so database tests do not silently skip there.
