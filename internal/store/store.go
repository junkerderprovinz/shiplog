// Package store persists per-container update status and version history in
// a local SQLite database (pure-Go modernc.org/sqlite driver).
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by Get when no status row matches the given id.
var ErrNotFound = errors.New("store: not found")

// Store is a SQLite-backed persistence layer for update status and history.
type Store struct {
	db *sql.DB
}

// HistoryEntry is a single recorded running-version change for a container.
type HistoryEntry struct {
	FromTag string
	ToTag   string
	SeenAt  time.Time
}

// AutoUpdateRecord is one auto-update ACTION ShipLog took (success or failure) —
// an audit trail distinct from the passive running-version `history`.
type AutoUpdateRecord struct {
	Name    string
	FromVer string
	ToVer   string
	Level   string
	Success bool
	Err     string
	At      int64 // unix seconds
}

const schema = `
CREATE TABLE IF NOT EXISTS status (
	container_id    TEXT PRIMARY KEY,
	name            TEXT,
	repo            TEXT,
	image           TEXT,
	tag             TEXT,
	digest          TEXT,
	pinned_digest   TEXT,
	is_local        INTEGER,
	running_version TEXT,
	newest_tag      TEXT,
	newest_digest   TEXT,
	kind            TEXT,
	risk            TEXT,
	risk_reason     TEXT,
	changelog_json  TEXT,
	checked_at      TEXT,
	error           TEXT
);
CREATE TABLE IF NOT EXISTS history (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	container_id TEXT,
	name         TEXT,
	from_tag     TEXT,
	to_tag       TEXT,
	seen_at      TEXT
);
CREATE TABLE IF NOT EXISTS autoupdate_log (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	name     TEXT,
	from_ver TEXT,
	to_ver   TEXT,
	level    TEXT,
	success  INTEGER,
	err      TEXT,
	at       INTEGER
);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT
);
CREATE TABLE IF NOT EXISTS source_overrides (
	repo       TEXT PRIMARY KEY,
	source     TEXT NOT NULL,
	updated_at INTEGER
);
`

// Open opens (creating if needed) the SQLite database at path, applies the
// pragmas, and ensures the schema exists.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: set WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: set busy_timeout: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: create schema: %w", err)
	}
	// Migrate older databases that predate later columns. On a fresh DB the
	// columns already exist, so the duplicate-column error is expected and
	// ignored.
	_, _ = db.Exec(`ALTER TABLE status ADD COLUMN running_version TEXT`)
	_, _ = db.Exec(`ALTER TABLE status ADD COLUMN pinned_digest TEXT`)
	_, _ = db.Exec(`ALTER TABLE status ADD COLUMN is_local INTEGER`)
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Upsert writes the status, and records a history row when the running version
// changed from a non-empty prior value. Keying off running_version (not the raw
// tag) means a ":latest" upgrade — where the tag stays "latest" but the resolved
// version moves 1.7.0 -> 1.8.0 — is recorded too.
func (s *Store) Upsert(st model.UpdateStatus) error {
	var priorVer string
	// COALESCE so a row migrated from a pre-running_version DB (column backfilled
	// NULL) scans into a string instead of erroring and aborting the upsert.
	err := s.db.QueryRow(`SELECT COALESCE(running_version, '') FROM status WHERE container_id = ?`, st.Container.ID).Scan(&priorVer)
	switch {
	case err == nil:
		if priorVer != "" && priorVer != st.RunningVersion && st.RunningVersion != "" {
			if _, err := s.db.Exec(
				`INSERT INTO history (container_id, name, from_tag, to_tag, seen_at) VALUES (?, ?, ?, ?, ?)`,
				st.Container.ID, st.Container.Name, priorVer, st.RunningVersion, st.CheckedAt.Format(time.RFC3339),
			); err != nil {
				return fmt.Errorf("store: append history: %w", err)
			}
		}
	case errors.Is(err, sql.ErrNoRows):
		// first time we see this container; no history to record
	default:
		return fmt.Errorf("store: read prior status: %w", err)
	}

	var changelogJSON string
	if st.Changelog != nil {
		b, err := json.Marshal(st.Changelog)
		if err != nil {
			return fmt.Errorf("store: marshal changelog: %w", err)
		}
		changelogJSON = string(b)
	}

	_, err = s.db.Exec(`
INSERT INTO status (
	container_id, name, repo, image, tag, digest, pinned_digest, is_local, running_version,
	newest_tag, newest_digest, kind, risk, risk_reason,
	changelog_json, checked_at, error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(container_id) DO UPDATE SET
	name            = excluded.name,
	repo            = excluded.repo,
	image           = excluded.image,
	tag             = excluded.tag,
	digest          = excluded.digest,
	pinned_digest   = excluded.pinned_digest,
	is_local        = excluded.is_local,
	running_version = excluded.running_version,
	newest_tag      = excluded.newest_tag,
	newest_digest   = excluded.newest_digest,
	kind            = excluded.kind,
	risk            = excluded.risk,
	risk_reason     = excluded.risk_reason,
	changelog_json  = excluded.changelog_json,
	checked_at      = excluded.checked_at,
	error           = excluded.error`,
		st.Container.ID, st.Container.Name, st.Container.Repo, st.Container.Image, st.Container.Tag, st.Container.Digest, st.Container.PinnedDigest, st.Container.IsLocal, st.RunningVersion,
		st.NewestTag, st.NewestDigest, string(st.Kind), string(st.Risk), st.RiskReason,
		changelogJSON, st.CheckedAt.Format(time.RFC3339), st.Error,
	)
	if err != nil {
		return fmt.Errorf("store: upsert status: %w", err)
	}
	return nil
}

// orderClause sorts by risk severity (high>medium>low>unknown>none) descending,
// then by name ascending.
const orderClause = `
ORDER BY CASE risk
	WHEN 'high'    THEN 4
	WHEN 'medium'  THEN 3
	WHEN 'low'     THEN 2
	WHEN 'unknown' THEN 1
	ELSE 0
END DESC, name ASC`

const selectCols = `
SELECT container_id, name, repo, image, tag, digest,
	COALESCE(pinned_digest, ''), COALESCE(is_local, 0), COALESCE(running_version, ''),
	newest_tag, newest_digest, kind, risk, risk_reason,
	changelog_json, checked_at, error
FROM status`

// List returns all status rows ordered by risk severity then name.
func (s *Store) List() ([]model.UpdateStatus, error) {
	rows, err := s.db.Query(selectCols + orderClause)
	if err != nil {
		return nil, fmt.Errorf("store: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.UpdateStatus
	for rows.Next() {
		st, err := scanStatus(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list iterate: %w", err)
	}
	return out, nil
}

// Get returns the status for id, or ErrNotFound.
func (s *Store) Get(id string) (model.UpdateStatus, error) {
	row := s.db.QueryRow(selectCols+` WHERE container_id = ?`, id)
	st, err := scanStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UpdateStatus{}, ErrNotFound
	}
	if err != nil {
		return model.UpdateStatus{}, err
	}
	return st, nil
}

// History returns the recorded version-change history for id, newest first.
func (s *Store) History(id string) ([]HistoryEntry, error) {
	rows, err := s.db.Query(
		`SELECT from_tag, to_tag, seen_at FROM history WHERE container_id = ? ORDER BY id DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("store: history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var seenAt string
		if err := rows.Scan(&e.FromTag, &e.ToTag, &seenAt); err != nil {
			return nil, fmt.Errorf("store: scan history: %w", err)
		}
		e.SeenAt, _ = time.Parse(time.RFC3339, seenAt)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: history iterate: %w", err)
	}
	return out, nil
}

// SetMeta upserts a single scalar key/value that must survive a restart (e.g.
// the last scheduled auto-update run time, so a reboot does not re-trigger an
// off-schedule run).
func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("store: set meta %q: %w", key, err)
	}
	return nil
}

// GetMeta returns the value for key, or "" (and no error) when the key is absent.
func (s *Store) GetMeta(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store: get meta %q: %w", key, err)
	}
	return v, nil
}

// SetSourceOverride records a user-chosen changelog source (a github.com repo
// URL) for an image repo, so the engine uses it instead of the image's OCI
// source label. repo is the normalised image repo (e.g. "lscr.io/linuxserver/radarr").
func (s *Store) SetSourceOverride(repo, source string) error {
	_, err := s.db.Exec(
		`INSERT INTO source_overrides (repo, source, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(repo) DO UPDATE SET source = excluded.source, updated_at = excluded.updated_at`,
		repo, source, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: set source override %q: %w", repo, err)
	}
	return nil
}

// Delete removes the status row for a container id (no error if absent). Used
// when "ignore third-party containers" drops a previously-tracked container so
// it disappears from the advisor.
func (s *Store) Delete(id string) error {
	if _, err := s.db.Exec(`DELETE FROM status WHERE container_id = ?`, id); err != nil {
		return fmt.Errorf("store: delete status %q: %w", id, err)
	}
	return nil
}

// DeleteSourceOverride removes any override for repo (no error if absent).
func (s *Store) DeleteSourceOverride(repo string) error {
	if _, err := s.db.Exec(`DELETE FROM source_overrides WHERE repo = ?`, repo); err != nil {
		return fmt.Errorf("store: delete source override %q: %w", repo, err)
	}
	return nil
}

// SourceOverrides returns all user overrides as a repo→source map (empty, not
// nil-erroring, when there are none).
func (s *Store) SourceOverrides() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT repo, source FROM source_overrides`)
	if err != nil {
		return nil, fmt.Errorf("store: list source overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()
	m := map[string]string{}
	for rows.Next() {
		var repo, source string
		if err := rows.Scan(&repo, &source); err != nil {
			return nil, fmt.Errorf("store: scan source override: %w", err)
		}
		m[repo] = source
	}
	return m, rows.Err()
}

// LogAutoUpdate appends one auto-update action to the audit log.
func (s *Store) LogAutoUpdate(rec AutoUpdateRecord) error {
	succ := 0
	if rec.Success {
		succ = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO autoupdate_log (name, from_ver, to_ver, level, success, err, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.Name, rec.FromVer, rec.ToVer, rec.Level, succ, rec.Err, rec.At)
	if err != nil {
		return fmt.Errorf("store: log auto-update: %w", err)
	}
	return nil
}

// AutoUpdateHistory returns up to limit recent auto-update actions, newest first.
func (s *Store) AutoUpdateHistory(limit int) ([]AutoUpdateRecord, error) {
	rows, err := s.db.Query(
		`SELECT name, from_ver, to_ver, level, success, err, at FROM autoupdate_log ORDER BY at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: auto-update history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AutoUpdateRecord
	for rows.Next() {
		var r AutoUpdateRecord
		var succ int
		if err := rows.Scan(&r.Name, &r.FromVer, &r.ToVer, &r.Level, &succ, &r.Err, &r.At); err != nil {
			return nil, fmt.Errorf("store: scan auto-update history: %w", err)
		}
		r.Success = succ != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: auto-update history iterate: %w", err)
	}
	return out, nil
}

// scanner abstracts *sql.Row and *sql.Rows for shared column scanning.
type scanner interface {
	Scan(dest ...any) error
}

func scanStatus(sc scanner) (model.UpdateStatus, error) {
	var (
		st            model.UpdateStatus
		kind, risk    string
		changelogJSON string
		checkedAt     string
		isLocal       int64 // SQLite has no bool; scan the 0/1 integer explicitly
	)
	err := sc.Scan(
		&st.Container.ID, &st.Container.Name, &st.Container.Repo, &st.Container.Image, &st.Container.Tag, &st.Container.Digest,
		&st.Container.PinnedDigest, &isLocal, &st.RunningVersion,
		&st.NewestTag, &st.NewestDigest, &kind, &risk, &st.RiskReason,
		&changelogJSON, &checkedAt, &st.Error,
	)
	if err != nil {
		return model.UpdateStatus{}, err
	}
	st.Container.IsLocal = isLocal != 0
	st.Kind = model.Kind(kind)
	st.Risk = model.RiskLevel(risk)
	if checkedAt != "" {
		st.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
	}
	if changelogJSON != "" {
		var cl model.Changelog
		if err := json.Unmarshal([]byte(changelogJSON), &cl); err != nil {
			return model.UpdateStatus{}, fmt.Errorf("store: unmarshal changelog: %w", err)
		}
		st.Changelog = &cl
	}
	return st, nil
}
