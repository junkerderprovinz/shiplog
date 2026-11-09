package bannerlog

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadyContainsSentinel(t *testing.T) {
	var b bytes.Buffer
	Ready(&b, "http://0.0.0.0:8484")
	out := b.String()
	// The CI smoke gate greps docker logs for exactly this phrase.
	if !strings.Contains(out, "SHIPLOG IS READY") {
		t.Fatalf("Ready output missing the readiness sentinel:\n%s", out)
	}
	if !strings.Contains(out, "listening on http://0.0.0.0:8484\n") {
		t.Fatalf("Ready output missing the listen address:\n%s", out)
	}
}

func TestReadyNamesASocketAsItIs(t *testing.T) {
	var b bytes.Buffer
	Ready(&b, "unix:/var/run/shiplog.sock")
	if !strings.Contains(b.String(), "listening on unix:/var/run/shiplog.sock\n") {
		t.Fatalf("Ready output misnames the socket:\n%s", b.String())
	}
}

func TestInitWrites(t *testing.T) {
	var b bytes.Buffer
	Init(&b)
	if b.Len() == 0 {
		t.Fatal("Init wrote nothing")
	}
}
