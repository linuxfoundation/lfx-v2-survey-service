// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

const opTimeout = 5 * time.Second

type Store struct{ pool *pgxpool.Pool }

type resourceConnectionKey struct{}
type stateQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Locked handlers reuse their session for DML, avoiding pool starvation when
// every available connection owns a resource lock. Callers must not retain
// this context beyond the release callback or use it concurrently.
func (s *Store) db(ctx context.Context) stateQuerier {
	if conn, ok := ctx.Value(resourceConnectionKey{}).(*pgxpool.Conn); ok {
		return conn
	}
	return s.pool
}

var _ domain.EventState = (*Store)(nil)

func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("SURVEY_STATE_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid survey state database configuration")
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = opTimeout
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, domain.ErrEventStateUnavailable
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, domain.ErrEventStateUnavailable
	}
	return &Store{pool: pool}, nil
}
func (s *Store) Close() { s.pool.Close() }
func (s *Store) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var ready bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM survey_state_cutover)").Scan(&ready)
	if err != nil || !ready {
		return errors.New("survey state cutover not verified")
	}
	return nil
}
func (s *Store) ApplySchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1950001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type binding struct {
	table, payload string
	keys           []string
	args           []any
}

func bind(key string) (binding, error) {
	kind, id, ok := strings.Cut(key, ".")
	if !ok || id == "" || strings.ContainsAny(key, " \t\r\n*>/\\") {
		return binding{}, jetstream.ErrInvalidKey
	}
	switch kind {
	case "survey", "survey_response", "survey_template":
		return binding{"survey_processed_resources", "legacy_uid", []string{"resource_kind", "source_id"}, []any{kind, id}}, nil
	case "v1_survey_response_lfid_invite_sent":
		return binding{"survey_invitation_guards", "invite_id", []string{"response_id"}, []any{id}}, nil
	default:
		return binding{}, errors.New("unowned survey state family")
	}
}
func (b binding) decode(value []byte) (string, string) {
	switch string(value) {
	case "":
		return "empty", ""
	case "!del":
		return "deleted", ""
	}
	if b.table == "survey_invitation_guards" {
		if string(value) == "pending" {
			return "pending", ""
		}
		return "sent", string(value)
	}
	if string(value) == "1" {
		return "processed", ""
	}
	return "legacy", string(value)
}
func where(b binding) string {
	parts := make([]string, len(b.keys))
	for i, col := range b.keys {
		parts[i] = fmt.Sprintf("%s=$%d", col, i+1)
	}
	return strings.Join(parts, " AND ")
}

type entry struct {
	key     string
	value   []byte
	version uint64
	updated time.Time
}

func (e entry) Bucket() string                  { return "survey-event-state" }
func (e entry) Key() string                     { return e.key }
func (e entry) Value() []byte                   { return e.value }
func (e entry) Revision() uint64                { return e.version }
func (e entry) Created() time.Time              { return e.updated }
func (e entry) Delta() uint64                   { return 0 }
func (e entry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }
func (s *Store) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	b, err := bind(key)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	e := entry{key: key}
	var state, payload string
	err = s.db(ctx).QueryRow(ctx, "SELECT state,"+b.payload+",version,updated_at FROM "+b.table+" WHERE "+where(b), b.args...).Scan(&state, &payload, &e.version, &e.updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, jetstream.ErrKeyNotFound
	}
	if err != nil {
		return nil, domain.ErrEventStateUnavailable
	}
	switch state {
	case "processed":
		e.value = []byte("1")
	case "pending":
		e.value = []byte("pending")
	case "deleted":
		e.value = []byte("!del")
	default:
		e.value = []byte(payload)
	}
	return e, nil
}
func (s *Store) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	return s.write(ctx, key, value, "put", 0)
}
func (s *Store) Create(ctx context.Context, key string, value []byte, opts ...jetstream.KVCreateOpt) (uint64, error) {
	if len(opts) > 0 {
		return 0, errors.New("state create options unsupported")
	}
	return s.write(ctx, key, value, "create", 0)
}
func (s *Store) Update(ctx context.Context, key string, value []byte, rev uint64) (uint64, error) {
	return s.write(ctx, key, value, "update", rev)
}
func (s *Store) write(ctx context.Context, key string, value []byte, op string, rev uint64) (uint64, error) {
	if err := domain.CheckSourceFence(ctx); err != nil {
		return 0, err
	}
	b, err := bind(key)
	if err != nil {
		return 0, err
	}
	state, payload := b.decode(value)
	args := append(append([]any{}, b.args...), state, payload)
	var query string
	if op == "update" {
		args = append(args, rev)
		query = "UPDATE " + b.table + fmt.Sprintf(" SET state=$%d,%s=$%d,version=nextval('survey_state_revision'),updated_at=now() WHERE ", len(b.args)+1, b.payload, len(b.args)+2) + where(b) + fmt.Sprintf(" AND version=$%d RETURNING version", len(args))
	} else {
		cols := append(append([]string{}, b.keys...), "state", b.payload)
		params := make([]string, len(args))
		for i := range params {
			params[i] = fmt.Sprintf("$%d", i+1)
		}
		query = "INSERT INTO " + b.table + " (" + strings.Join(cols, ",") + ") VALUES (" + strings.Join(params, ",") + ") ON CONFLICT (" + strings.Join(b.keys, ",") + ") "
		if op == "create" {
			query += "DO NOTHING"
		} else {
			query += "DO UPDATE SET state=EXCLUDED.state," + b.payload + "=EXCLUDED." + b.payload + ",version=nextval('survey_state_revision'),updated_at=now()"
		}
		query += " RETURNING version"
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var version uint64
	err = s.db(ctx).QueryRow(ctx, query, args...).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		if op == "create" {
			return 0, jetstream.ErrKeyExists
		}
		return 0, domain.ErrEventStateRevisionMismatch
	}
	if err != nil {
		return 0, domain.ErrEventStateUnavailable
	}
	return version, nil
}
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := domain.CheckSourceFence(ctx); err != nil {
		return err
	}
	b, err := bind(key)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	_, err = s.db(ctx).Exec(ctx, "DELETE FROM "+b.table+" WHERE "+where(b), b.args...)
	if err != nil {
		return domain.ErrEventStateUnavailable
	}
	return nil
}

func (s *Store) PrepareInvitation(ctx context.Context, id string) error {
	if err := domain.CheckSourceFence(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	_, err := s.db(ctx).Exec(ctx, "INSERT INTO survey_invitation_intents(response_id) VALUES($1) ON CONFLICT DO NOTHING", id)
	if err != nil {
		return domain.ErrEventStateUnavailable
	}
	return nil
}
func (s *Store) InvitationPrepared(ctx context.Context, id string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var prepared bool
	err := s.db(ctx).QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM survey_invitation_intents WHERE response_id=$1 AND NOT resolved)", id).Scan(&prepared)
	if err != nil {
		return false, domain.ErrEventStateUnavailable
	}
	return prepared, nil
}

func (s *Store) ResolveInvitation(ctx context.Context, id string) error {
	if err := domain.CheckSourceFence(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	_, err := s.db(ctx).Exec(ctx, "INSERT INTO survey_invitation_intents(response_id,resolved) VALUES($1,true) ON CONFLICT(response_id) DO UPDATE SET resolved=true", id)
	if err != nil {
		return domain.ErrEventStateUnavailable
	}
	return nil
}

// LockResource owns a session advisory lock; failed unlocks never return a
// potentially locked connection to the pool. Hash collisions only serialize.
func (s *Store) LockResource(ctx context.Context, key string) (context.Context, func(), error) {
	acquireCtx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	conn, err := s.pool.Acquire(acquireCtx)
	if err != nil {
		return ctx, nil, domain.ErrEventStateUnavailable
	}
	var locked bool
	err = conn.QueryRow(acquireCtx, "SELECT pg_try_advisory_lock(hashtextextended($1,1950068))", key).Scan(&locked)
	if err != nil {
		raw := conn.Hijack()
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), opTimeout)
		defer cleanupCancel()
		_ = raw.Close(cleanup)
		return ctx, nil, domain.ErrEventStateUnavailable
	}
	if !locked {
		conn.Release()
		return ctx, nil, domain.ErrResourceBusy
	}
	return context.WithValue(ctx, resourceConnectionKey{}, conn), func() {
		cleanup, cancel := context.WithTimeout(context.Background(), opTimeout)
		defer cancel()
		var unlocked bool
		err := conn.QueryRow(cleanup, "SELECT pg_advisory_unlock(hashtextextended($1,1950068))", key).Scan(&unlocked)
		if err != nil || !unlocked {
			raw := conn.Hijack()
			_ = raw.Close(cleanup)
			return
		}
		conn.Release()
	}, nil
}
