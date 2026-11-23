package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/config"
)

func serve(t *testing.T, ln net.Listener) {
	t.Helper()
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		ReadHeaderTimeout: time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

func TestTheAPIAnswersOnItsSocketAlone(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "shiplog.sock")
	ln, _, err := listen(config.Config{Socket: sock})
	if err != nil {
		t.Fatal(err)
	}
	serve(t, ln)
	if ln.Addr().Network() != "unix" {
		t.Fatalf("the API listens on %s", ln.Addr().Network())
	}

	hc := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}}
	resp, err := hc.Get("http://shiplog/")
	if err != nil {
		t.Fatalf("the API does not answer on its socket: %v", err)
	}
	_ = resp.Body.Close()
}

func TestSocketListenerReplacesAStaleSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "shiplog.sock")
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ln, _, err := listen(config.Config{Socket: sock})
	if err != nil {
		t.Fatalf("a socket left by an unclean stop blocks the start: %v", err)
	}
	_ = ln.Close()
}

func TestSocketIsOpenToRootAndItsGroupOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no unix file modes")
	}
	sock := filepath.Join(t.TempDir(), "shiplog.sock")
	ln, _, err := listen(config.Config{Socket: sock})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	fi, err := os.Stat(sock)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o660 {
		t.Fatalf("socket mode %o, want 660", got)
	}
}
