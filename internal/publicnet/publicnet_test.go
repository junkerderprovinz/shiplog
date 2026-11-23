package publicnet

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOnlyGloballyRoutableAddressesArePublic(t *testing.T) {
	for _, s := range []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.20.80", "169.254.169.254", "0.0.0.0", "100.64.0.1",
		"224.0.0.1", "255.255.255.255", "::1", "::", "fe80::1", "fc00::1", "::ffff:127.0.0.1", "::ffff:192.168.1.1",
		"64:ff9b:1::7f00:1", "2001::1", "2002:7f00:1::1", "2001:db8::1", "ff02::1",
	} {
		if PublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s counts as public", s)
		}
	}
	for _, s := range []string{"185.199.108.133", "140.82.121.4", "1.1.1.1", "2606:4700::1111", "2a01:4f8::1"} {
		if !PublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s counts as inside", s)
		}
	}
}

func TestCheckRefusesAnAddressThatCannotBeFetched(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/CHANGELOG.md", "http://[::1]:8080/x", "http://169.254.169.254/latest/meta-data",
		"http://192.168.20.11/History.txt", "ftp://example.org/CHANGES", "file:///etc/passwd",
		"https://user:secret@example.org/CHANGES.md", "https:///CHANGES.md",
	} {
		if err := Check(raw); err == nil {
			t.Errorf("%s passes", raw)
		}
	}
	for _, raw := range []string{"https://raw.githubusercontent.com/o/r/main/CHANGELOG.md", "http://example.org/History.txt"} {
		if err := Check(raw); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	if err := Check("http://10.0.0.1/x"); !errors.Is(err, ErrPrivate) {
		t.Errorf("an address inside gives %v, want ErrPrivate", err)
	}
}

func TestTheClientRefusesAServerOnThisMachine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer srv.Close()
	resp, err := Client(5 * time.Second).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the client reached a server on 127.0.0.1")
	}
	if !errors.Is(err, ErrPrivate) {
		t.Errorf("got %v, want ErrPrivate", err)
	}
}

func TestARedirectIntoTheLocalNetworkIsRefused(t *testing.T) {
	inside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer inside.Close()
	outside := httptest.NewServer(http.RedirectHandler(inside.URL+"/admin", http.StatusFound))
	defer outside.Close()
	// The outside server stands in for a public one: only its port counts as public here.
	port := strings.TrimPrefix(outside.URL, "http://127.0.0.1:")
	c := client(5*time.Second, func(a netip.AddrPort) bool { return a.String() == "127.0.0.1:"+port })
	resp, err := c.Get(outside.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the redirect reached the server inside")
	}
	if !errors.Is(err, ErrPrivate) {
		t.Errorf("got %v, want ErrPrivate", err)
	}
}

func TestRedirectsStopAfterFiveAndStayOnTheWeb(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ftp" {
			http.Redirect(w, r, "ftp://example.org/x", http.StatusFound)
			return
		}
		http.Redirect(w, r, srv.URL+r.URL.Path+"x", http.StatusFound)
	}))
	defer srv.Close()
	c := client(5*time.Second, func(netip.AddrPort) bool { return true })
	for _, path := range []string{"/loop", "/ftp"} {
		resp, err := c.Get(srv.URL + path)
		if err == nil {
			_ = resp.Body.Close()
			t.Errorf("%s: followed", path)
		}
	}
}

func TestTheClientGoesThroughNoProxy(t *testing.T) {
	tr, ok := Client(time.Second).Transport.(*http.Transport)
	if !ok {
		t.Fatal("the client has no http.Transport")
	}
	if tr.Proxy != nil {
		p, _ := tr.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.org"}})
		t.Errorf("the client uses a proxy: %v", p)
	}
}
