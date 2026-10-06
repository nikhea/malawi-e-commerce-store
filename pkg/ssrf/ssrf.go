// Package ssrf guards server-side URL fetching (media worker pulling a
// SourceURL, future webhook/asset fetchers). An attacker who controls a
// URL must never reach internal targets: private ranges, loopback, link
// local (cloud metadata 169.254.169.254 lives here), or resolve tricks.
//
// Design limits (documented, not hidden): validation is check-then-fetch
// (a DNS-rebinding race remains — acceptable at this scale; the fix is a
// resolving dialer pinning the validated IP). Hostnames resolve via the
// system resolver; literal IPs are checked directly.
package ssrf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ValidateURL rejects URLs unsafe for server-side fetching. Resolution
// uses a 5s timeout so a hanging resolver can't stall a worker job.
func ValidateURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return errors.New("ssrf: invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("ssrf: scheme %q not allowed", u.Scheme)
	}
	if u.User != nil {
		return errors.New("ssrf: credentials in url not allowed")
	}

	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		return checkIP(ip)
	}
	// Hostname: resolve every address (rebinding can rotate answers;
	// checking all current answers is the practical defense).
	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("ssrf: cannot resolve host: %w", err)
	}
	for _, a := range addrs {
		if err := checkIP(a.IP); err != nil {
			return err
		}
	}
	return nil
}

func checkIP(ip net.IP) error {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || !ip.IsGlobalUnicast() {
		return fmt.Errorf("ssrf: non-routable address %s", ip)
	}
	// RFC1918 + CGNAT + documentation ranges IsGlobalUnicast misses.
	for _, cidr := range []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"100.64.0.0/10", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return fmt.Errorf("ssrf: private address %s", ip)
		}
	}
	return nil
}
