package bannerlog

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadyContainsSentinel(t *testing.T) {
	var b bytes.Buffer
	Ready(&b, "0.0.0.0:8484")
	out := b.String()
	// The CI smoke gate greps docker logs for exactly this phrase.
	if !strings.Contains(out, "SHIPLOG IS READY") {
		t.Fatalf("Ready output missing the readiness sentinel:\n%s", out)
	}
	if !strings.Contains(out, "0.0.0.0:8484") {
		t.Fatalf("Ready output missing the listen address:\n%s", out)
	}
}

func TestInitWrites(t *testing.T) {
	var b bytes.Buffer
	Init(&b)
	if b.Len() == 0 {
		t.Fatal("Init wrote nothing")
	}
}
