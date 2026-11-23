package gate

import (
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

const testPassword = "passwort1234"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

var page = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><title>ShipLog</title>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

// engine stands in for ShipLog's API: the status page, its JSON and a change.
func engine() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>status</html>")) })
	mux.HandleFunc("GET /logo.svg", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<svg/>")) })
	mux.HandleFunc("GET /api/containers", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("[]")) })
	mux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	return mux
}

func serve(t *testing.T, s Settings) http.Handler {
	t.Helper()
	h, closer, err := New(s, engine(), page, quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closer() })
	return h
}

func send(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func setPassword(t *testing.T, h http.Handler) {
	t.Helper()
	if w := send(h, http.MethodPut, "/api/signin/v1/me/password", `{"next":"`+testPassword+`"}`); w.Code != http.StatusOK {
		t.Fatalf("first password: %d %s", w.Code, w.Body)
	}
}

func signInCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	w := send(h, http.MethodPost, "/api/signin/v1/login", `{"password":"`+testPassword+`"}`)
	for _, c := range w.Result().Cookies() {
		if strings.HasSuffix(c.Name, "_session") {
			return c
		}
	}
	t.Fatalf("no session after signing in: %d %s", w.Code, w.Body)
	return nil
}

func TestTheFirstStartDrawsAKeyAndKeepsIt(t *testing.T) {
	dir := t.TempDir()
	serve(t, Settings{Dir: dir})
	first, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		t.Fatal(err)
	}
	if key, err := hex.DecodeString(strings.TrimSpace(string(first))); err != nil || len(key) != 32 {
		t.Fatalf("key file holds %q", first)
	}
	if info, _ := os.Stat(filepath.Join(dir, keyFile)); os.PathSeparator == '/' && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("key file mode %v", info.Mode())
	}
	serve(t, Settings{Dir: dir})
	if again, _ := os.ReadFile(filepath.Join(dir, keyFile)); string(again) != string(first) {
		t.Fatal("a second start drew another key")
	}
}

func TestAKeyFromTheEnvironmentLeavesNoKeyFile(t *testing.T) {
	dir := t.TempDir()
	serve(t, Settings{Dir: dir, Key: strings.Repeat("ab", 32)})
	if _, err := os.Stat(filepath.Join(dir, keyFile)); !os.IsNotExist(err) {
		t.Fatalf("key file written beside a key from the environment: %v", err)
	}
}

func TestAKeyThatIsNotSixtyFourHexCharactersStopsTheStart(t *testing.T) {
	if _, _, err := New(Settings{Dir: t.TempDir(), Key: "short"}, engine(), page, quiet); err == nil {
		t.Fatal("a short key started the server")
	}
}

func TestAStoreSealedWithAnotherKeyStopsTheStart(t *testing.T) {
	dir := t.TempDir()
	h := serve(t, Settings{Dir: dir})
	setPassword(t, h)
	if err := os.WriteFile(filepath.Join(dir, keyFile), []byte(strings.Repeat("cd", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := New(Settings{Dir: dir}, engine(), page, quiet); err == nil {
		t.Fatal("a store that does not open looked like an app without a password")
	}
}

func TestWithoutAPasswordEveryRouteAnswersAsBefore(t *testing.T) {
	h := serve(t, Settings{Dir: t.TempDir()})
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/", 200}, {"GET", "/logo.svg", 200}, {"GET", "/api/containers", 200},
		{"POST", "/api/refresh", 202}, {"GET", "/account/", 200},
	} {
		if w := send(h, tc.method, tc.path, ""); w.Code != tc.want {
			t.Errorf("%s %s on an open ShipLog: %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}

func TestAPasswordClosesEverythingButTheWayIn(t *testing.T) {
	h := serve(t, Settings{Dir: t.TempDir()})
	setPassword(t, h)

	for _, p := range []string{"/account/", "/account/assets/app.js", "/logo.svg"} {
		if w := send(h, "GET", p, ""); w.Code != http.StatusOK {
			t.Errorf("GET %s before signing in: %d, want 200", p, w.Code)
		}
	}
	if w := send(h, "GET", "/", ""); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/account/" {
		t.Errorf("GET / before signing in: %d to %q, want the way to /account/", w.Code, w.Header().Get("Location"))
	}
	for _, tc := range []struct{ method, path string }{{"GET", "/api/containers"}, {"POST", "/api/refresh"}} {
		if w := send(h, tc.method, tc.path, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s before signing in: %d, want 401", tc.method, tc.path, w.Code)
		}
	}
	cookie := signInCookie(t, h)
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"GET", "/", 200}, {"GET", "/api/containers", 200}} {
		if w := send(h, tc.method, tc.path, "", cookie); w.Code != tc.want {
			t.Errorf("%s %s signed in: %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}

func TestTheSignInKeepsTheContract(t *testing.T) {
	signintest.Run(t, serve(t, Settings{Dir: t.TempDir()}), signintest.Options{Password: testPassword})
}

func TestForwardedHeadersCountOnlyFromATrustedProxy(t *testing.T) {
	for _, tc := range []struct {
		trust string
		want  bool
	}{{"", false}, {"false", false}, {"true", true}, {"10.0.0.0/8, 192.168.1.2", true}} {
		peer, prefixes, err := Settings{TrustProxy: tc.trust}.proxies()
		if err != nil {
			t.Fatalf("TRUST_PROXY=%q: %v", tc.trust, err)
		}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.1.2.3:4000"
		got := peer != nil && peer(r) || len(prefixes) > 0 && prefixes[0].Contains(netip.MustParseAddr("10.1.2.3"))
		if got != tc.want {
			t.Errorf("TRUST_PROXY=%q trusts 10.1.2.3: %v, want %v", tc.trust, got, tc.want)
		}
	}
	if _, _, err := (Settings{TrustProxy: "proxy.lan"}).proxies(); err == nil {
		t.Error("a name in TRUST_PROXY was taken")
	}
}

func TestThePublicAddressComesOnlyFromBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                            "",
		"not a url":                   "",
		"ftp://sl.example":            "",
		"https://SL.example:443/sub/": "https://sl.example",
		"http://192.168.1.5:8484":     "http://192.168.1.5:8484",
	} {
		got := ""
		if u := (Settings{BaseURL: in}).public(); u != nil {
			got = u.String()
		}
		if got != want {
			t.Errorf("BASE_URL=%q: %q, want %q", in, got, want)
		}
	}
}

func TestABuildWithoutThePageListsNoFolder(t *testing.T) {
	h, closer, err := New(Settings{Dir: t.TempDir()}, engine(), fstest.MapFS{".gitkeep": {}}, quiet)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer() }()
	if w := send(h, "GET", "/account/", ""); w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), ".gitkeep") {
		t.Errorf("/account/ without a built page: %d %s", w.Code, w.Body)
	}
}
