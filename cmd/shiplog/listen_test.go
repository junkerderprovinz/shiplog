package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/config"
)

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func serve(t *testing.T, ln net.Listener) {
	t.Helper()
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		ReadHeaderTimeout: time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

func TestSocketListenerOpensNoTCPPort(t *testing.T) {
	port := freePort(t)
	sock := filepath.Join(t.TempDir(), "shiplog.sock")
	ln, _, err := listen(config.Config{Port: port, Socket: sock})
	if err != nil {
		t.Fatal(err)
	}
	serve(t, ln)

	if c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, time.Second); err == nil {
		_ = c.Close()
		t.Fatalf("the API answers on TCP port %s", port)
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

func TestWithoutSocketTheAPIListensOnPort(t *testing.T) {
	port := freePort(t)
	ln, _, err := listen(config.Config{Port: port})
	if err != nil {
		t.Fatal(err)
	}
	serve(t, ln)
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, time.Second)
	if err != nil {
		t.Fatalf("the container image's API is not on port %s: %v", port, err)
	}
	_ = c.Close()
}
