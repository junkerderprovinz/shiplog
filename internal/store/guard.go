package store

import "fmt"

// Guard is what ShipLog keeps about a container's update while there is a
// backup app in the fortress: why the update waits, or what the container ran
// before it, to go back to.
type Guard struct {
	Name  string
	State string
	// Repository is the image the template ran before the update, Digest the
	// manifest digest the container ran.
	Repository string
	Digest     string
	Snapshot   string
	Link       string // where the backup app shows the snapshot
	Reason     string
	At         int64 // unix seconds
}

// The states of a Guard.
const (
	GuardNoBackup   = "no_backup"   // the update waits, there was no backup
	GuardUpdating   = "updating"    // the old image is held while the update runs
	GuardRolledBack = "rolled_back" // the update failed and the container runs its old image
)

// SetGuard keeps g as the one guard of its container.
func (s *Store) SetGuard(g Guard) error {
	_, err := s.db.Exec(
		`INSERT INTO guards (name, state, repository, digest, snapshot, link, reason, at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET state = excluded.state, repository = excluded.repository, digest = excluded.digest,
		 snapshot = excluded.snapshot, link = excluded.link, reason = excluded.reason, at = excluded.at`,
		g.Name, g.State, g.Repository, g.Digest, g.Snapshot, g.Link, g.Reason, g.At)
	if err != nil {
		return fmt.Errorf("store: set guard %q: %w", g.Name, err)
	}
	return nil
}

// Guards returns every guard by container name.
func (s *Store) Guards() ([]Guard, error) {
	rows, err := s.db.Query(`SELECT name, state, repository, digest, snapshot, link, reason, at FROM guards ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: guards: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Guard
	for rows.Next() {
		var g Guard
		if err := rows.Scan(&g.Name, &g.State, &g.Repository, &g.Digest, &g.Snapshot, &g.Link, &g.Reason, &g.At); err != nil {
			return nil, fmt.Errorf("store: scan guard: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeleteGuard forgets the guard of the named container.
func (s *Store) DeleteGuard(name string) error {
	if _, err := s.db.Exec(`DELETE FROM guards WHERE name = ?`, name); err != nil {
		return fmt.Errorf("store: delete guard %q: %w", name, err)
	}
	return nil
}
