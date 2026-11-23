// Package gate puts the kit's sign-in in front of ShipLog's engine when it
// runs as a container and answers on a port. Without a password ShipLog stays
// open as it always was; once one is set, only the page that signs in and the
// logo answer to a stranger.
package gate

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/parleyport/kit/signin"
	"github.com/junkerderprovinz/parleyport/kit/signin/filestore"
)

const (
	// keyFile holds the key the sign-in is sealed with when SHIPLOG_APP_KEY is
	// not set. A copy of the folder carries both, which is why the variable
	// outside it is the stronger choice.
	keyFile   = "app.key"
	storeFile = "signin.json"
)

// Page is where the page that signs in and the sign-in settings live.
const Page = "/account/"

// Settings are what the container reads from its environment.
type Settings struct {
	Dir        string // DATA_DIR, where the key and the accounts are kept
	Key        string // SHIPLOG_APP_KEY, 64 hexadecimal characters
	BaseURL    string // BASE_URL, where people reach ShipLog
	TrustProxy string // TRUST_PROXY: true, false or addresses and CIDR ranges
}

// New opens the sign-in in s.Dir and returns app behind it, with the kit's
// routes and the page in ui under Page.
func New(s Settings, app http.Handler, ui fs.FS, log *slog.Logger) (http.Handler, func() error, error) {
	auth, err := open(s, log)
	if err != nil {
		return nil, nil, err
	}
	mux := http.NewServeMux()
	mux.Handle(signin.APIPrefix, auth.Handler())
	mux.Handle(signin.BrowserPrefix, auth.Handler())
	mux.Handle(Page, http.StripPrefix(strings.TrimSuffix(Page, "/"), pageFiles(ui)))
	mux.Handle("GET /logo.svg", app)
	mux.Handle("/", closed(app))
	return auth.Guard(mux), auth.Close, nil
}

func open(s Settings, log *slog.Logger) (*signin.Auth, error) {
	key, err := loadKey(s)
	if err != nil {
		return nil, err
	}
	seal, err := signin.NewSealer(key)
	if err != nil {
		return nil, err
	}
	store, err := filestore.Open(filepath.Join(s.Dir, storeFile), seal)
	if err != nil {
		return nil, err
	}
	peer, prefixes, err := s.proxies()
	if err != nil {
		return nil, err
	}
	return signin.Open(signin.Config{
		Domain:      "sl",
		Name:        "ShipLog",
		Store:       store,
		Seal:        seal,
		Owner:       true,
		TokenPrefix: "sl",
		Public:      s.public,
		Proxies:     prefixes,
		TrustedPeer: peer,
		Logger:      log,
	})
}

// closed lets a visitor nobody signed in for no further than the way to the
// page that signs in.
func closed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := signin.From(r.Context()); ok {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			http.Redirect(w, r, Page, http.StatusSeeOther)
			return
		}
		http.Error(w, "Please sign in first.", http.StatusUnauthorized)
	})
}

// pageFiles serves the built page. Vite names everything under assets/ by
// its content, so those keep for good and index.html is asked for again.
func pageFiles(ui fs.FS) http.Handler {
	if _, err := fs.Stat(ui, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "This build of ShipLog has no sign-in page; the container image has one.", http.StatusNotFound)
		})
	}
	files := http.FileServer(http.FS(ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

// loadKey returns the key from SHIPLOG_APP_KEY, or from the key file in the
// data folder, drawing and writing one on the first start.
func loadKey(s Settings) ([]byte, error) {
	if s.Key != "" {
		key, err := parseKey(s.Key)
		if err != nil {
			return nil, fmt.Errorf("SHIPLOG_APP_KEY: %w", err)
		}
		return key, nil
	}
	path := filepath.Join(s.Dir, keyFile)
	body, err := os.ReadFile(path)
	if err == nil {
		key, err := parseKey(strings.TrimSpace(string(body)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	// O_EXCL, so two processes starting at once cannot each write their own.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadKey(s)
	}
	if err != nil {
		return nil, fmt.Errorf("write %s, the data folder has to be writable for ShipLog: %w", path, err)
	}
	_, werr := f.WriteString(hex.EncodeToString(key) + "\n")
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write %s: %w", path, werr)
	}
	return key, nil
}

func parseKey(raw string) ([]byte, error) {
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("the key has to be 64 hexadecimal characters, as openssl rand -hex 32 prints")
	}
	return key, nil
}

// public is BASE_URL as browsers write it: a lower-case host without the
// default port, and no path. Passkeys and sign-in services need it.
func (s Settings) public() *url.URL {
	u, err := url.Parse(s.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return &url.URL{Scheme: u.Scheme, Host: host}
}

// proxies reads TRUST_PROXY: true trusts every peer, and a list names the
// proxies whose X-Forwarded-For counts.
func (s Settings) proxies() (func(*http.Request) bool, []netip.Prefix, error) {
	if s.TrustProxy == "" {
		return nil, nil, nil
	}
	if all, err := strconv.ParseBool(s.TrustProxy); err == nil {
		if all {
			return func(*http.Request) bool { return true }, nil, nil
		}
		return nil, nil, nil
	}
	var out []netip.Prefix
	for entry := range strings.SplitSeq(s.TrustProxy, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			p, err := netip.ParsePrefix(entry)
			if err != nil {
				return nil, nil, fmt.Errorf("TRUST_PROXY: %q is neither an address nor a CIDR range", entry)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, nil, fmt.Errorf("TRUST_PROXY: %q is neither an address nor a CIDR range", entry)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return nil, out, nil
}
