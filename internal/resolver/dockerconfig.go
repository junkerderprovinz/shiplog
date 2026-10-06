package resolver

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"sort"
	"strings"
)

// registryLogin is one login from Docker's config.json. It has no String
// method, so it cannot end up in a log line by accident.
type registryLogin struct {
	user   string
	secret string
}

// dockerHubAliases are the names config.json uses for Docker Hub.
var dockerHubAliases = map[string]bool{
	"index.docker.io":      true,
	"docker.io":            true,
	"registry.docker.io":   true,
	"registry-1.docker.io": true,
}

// loadDockerConfig returns the inline logins of the config.json at path, keyed
// by registry host. A missing or broken file yields none; anonymous access is
// always the fallback.
func loadDockerConfig(path string) map[string]registryLogin {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseDockerConfig(data)
}

// parseDockerConfig reads only secrets stored in the file itself. With a
// credsStore or credHelpers the auths entries are empty placeholders for an
// external helper, and the resolver never runs one.
func parseDockerConfig(data []byte) map[string]registryLogin {
	var cfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	// Several keys can name one host ("docker.io", "https://index.docker.io/v1/"),
	// so sorted order makes the winner deterministic.
	keys := make([]string, 0, len(cfg.Auths))
	for k := range cfg.Auths {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out map[string]registryLogin
	for _, k := range keys {
		host := registryHostFromAuthKey(k)
		if host == "" {
			continue
		}
		if _, dup := out[host]; dup {
			continue
		}
		e := cfg.Auths[k]
		user, secret, ok := decodeBasicAuth(e.Auth)
		if !ok {
			user, secret = e.Username, e.Password
		}
		if user == "" || secret == "" {
			continue
		}
		if out == nil {
			out = map[string]registryLogin{}
		}
		out[host] = registryLogin{user: user, secret: secret}
	}
	return out
}

// decodeBasicAuth splits the base64 "user:pass" of an auth field at the first
// colon, since passwords may contain colons.
func decodeBasicAuth(auth string) (user, secret string, ok bool) {
	if auth == "" {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", false
	}
	user, secret, found := strings.Cut(string(raw), ":")
	if !found || user == "" || secret == "" {
		return "", "", false
	}
	return user, secret, true
}

// registryHostFromAuthKey turns an auths key (a host, or a URL such as
// "https://index.docker.io/v1/") into the resolver's host name.
func registryHostFromAuthKey(key string) string {
	h := strings.ToLower(strings.TrimSpace(key))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexByte(h, '/'); i >= 0 {
		h = h[:i]
	}
	if dockerHubAliases[h] {
		return dockerHubRegistry
	}
	return h
}
