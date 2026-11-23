// Package publicnet fetches addresses a user typed in, and only those on the
// public internet, so a changelog source cannot be turned into a way to read
// what only the server reaches, such as a router's page or ShipLog's own API.
package publicnet

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrPrivate is the refusal for an address in the local network or on the
// server itself.
var ErrPrivate = errors.New("addresses in the local network or on the server itself cannot be read")

// globalV6 is the only part of the IPv6 space handed out for the internet.
// Outside it lie the forms that carry an IPv4 address, such as ::7f00:1 and
// 64:ff9b:1::/48, and retired ones such as fec0::/10.
var globalV6 = netip.MustParsePrefix("2000::/3")

// blocked are the special ranges netip does not name. Teredo and 6to4 can
// tunnel to an IPv4 address of the sender's choosing.
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

// PublicAddr reports whether a is globally routable.
func PublicAddr(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
		return false
	}
	if a.Is6() && !globalV6.Contains(a) {
		return false
	}
	for _, p := range blocked {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// Check refuses an address before any request: one that is not http or
// https, carries a user name or password, or names an address inside. A name
// is checked once it resolves, by the Client.
func Check(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return errors.New("this is not a web address")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("only http and https addresses can be read")
	}
	if u.User != nil {
		return errors.New("an address with a user name or password cannot be read")
	}
	if a, err := netip.ParseAddr(u.Hostname()); err == nil && !PublicAddr(a) {
		return ErrPrivate
	}
	return nil
}

// Client returns an HTTP client that connects to public addresses only. It
// checks every connection after the name resolved, so a name that points
// inside, a redirect there and a DNS answer that changes in between are all
// caught. It follows five redirects at most and uses no proxy, which would
// make the check meaningless.
func Client(timeout time.Duration) *http.Client {
	return client(timeout, func(a netip.AddrPort) bool { return PublicAddr(a.Addr()) })
}

func client(timeout time.Duration, public func(netip.AddrPort) bool) *http.Client {
	guard := func(_, address string, _ syscall.RawConn) error {
		a, err := netip.ParseAddrPort(address)
		if err != nil || !public(a) {
			return ErrPrivate
		}
		return nil
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second, Control: guard}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ForceAttemptHTTP2:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("the address redirects too often")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("the address redirects to something other than http or https")
			}
			return nil
		},
	}
}
