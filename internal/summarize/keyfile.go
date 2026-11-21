package summarize

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrBadKey refuses a value that cannot be an API key.
var ErrBadKey = errors.New("not an API key")

// KeyFile keeps an API key in a file only root can read. The settings page
// writes it and never reads it back, so the key leaves the server only in the
// request that uses it.
type KeyFile struct {
	Path string
}

// Load returns the key, or "" when none is saved.
func (k KeyFile) Load() (string, error) {
	b, err := os.ReadFile(k.Path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("cannot read the key file")
	}
	return strings.TrimSpace(string(b)), nil
}

// Save replaces the key. It goes through a temporary file, so a failed write
// leaves the old key in place.
func (k KeyFile) Save(key string) error {
	key = strings.TrimSpace(key)
	if !plausibleKey(key) {
		return ErrBadKey
	}
	tmp, err := os.CreateTemp(filepath.Dir(k.Path), ".key-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(key + "\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), k.Path); err != nil {
		return err
	}
	return os.Chmod(k.Path, 0o600)
}

func (k KeyFile) Remove() error {
	if err := os.Remove(k.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Hint reports whether a key is saved and its last four characters, which is
// all the settings page shows of it.
func (k KeyFile) Hint() (saved bool, last4 string) {
	key, err := k.Load()
	if err != nil || key == "" {
		return false, ""
	}
	if len(key) < 8 {
		return true, ""
	}
	return true, key[len(key)-4:]
}

// plausibleKey takes printable ASCII without spaces, which also keeps a line
// break from reaching the Authorization header.
func plausibleKey(s string) bool {
	if len(s) < 8 || len(s) > 512 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}
