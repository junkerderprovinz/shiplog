package summarize

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAKeyFileKeepsTheKeyForRootAlone(t *testing.T) {
	kf := KeyFile{Path: filepath.Join(t.TempDir(), "openrouter.key")}
	if err := kf.Save("  " + testKey + "\n"); err != nil {
		t.Fatal(err)
	}
	got, err := kf.Load()
	if err != nil || got != testKey {
		t.Fatalf("Load = %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(kf.Path)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, want 0600", st.Mode().Perm())
		}
	}
	if saved, hint := kf.Hint(); !saved || hint != "cdef" {
		t.Errorf("Hint = %v %q, want the last four characters", saved, hint)
	}
}

func TestAKeyFileWithoutAKeyLoadsNothing(t *testing.T) {
	kf := KeyFile{Path: filepath.Join(t.TempDir(), "openrouter.key")}
	if got, err := kf.Load(); got != "" || err != nil {
		t.Errorf("Load = %q, %v", got, err)
	}
	if saved, hint := kf.Hint(); saved || hint != "" {
		t.Errorf("Hint = %v %q", saved, hint)
	}
	if err := kf.Remove(); err != nil {
		t.Errorf("removing a key that is not there: %v", err)
	}
}

func TestAKeyFileRefusesWhatCannotBeAKey(t *testing.T) {
	kf := KeyFile{Path: filepath.Join(t.TempDir(), "openrouter.key")}
	for _, bad := range []string{"", "   ", "sk-or v1", "sk-or\r\nX-Evil: 1", "short", strings.Repeat("k", 600)} {
		if err := kf.Save(bad); err != ErrBadKey {
			t.Errorf("Save(%q) = %v, want ErrBadKey", bad, err)
		}
	}
	if _, err := os.Stat(kf.Path); !os.IsNotExist(err) {
		t.Error("a refused key must leave no file")
	}
}

func TestRemovingTheKeyDeletesItsFile(t *testing.T) {
	kf := KeyFile{Path: filepath.Join(t.TempDir(), "openrouter.key")}
	if err := kf.Save(testKey); err != nil {
		t.Fatal(err)
	}
	if err := kf.Remove(); err != nil {
		t.Fatal(err)
	}
	if saved, _ := kf.Hint(); saved {
		t.Error("the key is still there")
	}
}
