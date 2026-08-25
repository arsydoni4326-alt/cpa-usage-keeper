// Package enrichgeo provides an optional, opt-in, privacy-preserving
// enrichment layer for client IPs (e.g. a session's login_ip / last_seen_ip).
//
// The only built-in provider is a reverse-DNS (PTR) lookup implemented with
// the standard library, so no third-party service or dependency is required
// and no data leaves the host beyond an ordinary DNS query. Results are held
// in an in-memory cache with a TTL and are never persisted. Private, loopback,
// link-local and other reserved addresses are classified locally and are never
// sent to the resolver, so internal host details cannot leak through the UI.
package enrichgeo

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Enrichment describes the optional metadata resolved for a single client IP.
type Enrichment struct {
	// Enabled reports whether the enrichment feature is switched on. False
	// means the operator opted out and consumers must not offer any label.
	Enabled bool `json:"enabled"`
	// Hostname is the reverse-DNS (PTR) hostname for a public address, when
	// one is published. A trailing dot, if any, is stripped.
	Hostname string `json:"hostname,omitempty"`
	// Private marks a loopback / private / link-local / reserved address.
	// Such addresses are classified locally and are never resolved.
	Private bool `json:"private,omitempty"`
	// Pending is true while a background resolution is still in flight (first
	// read after a cache miss). Consumers should treat it as "resolving…".
	Pending bool `json:"pending,omitempty"`
}

// Resolver resolves enrichment metadata for a single public address.
type Resolver interface {
	Resolve(ctx context.Context, addr netip.Addr) (Enrichment, error)
}

// Options controls the enrichment layer.
type Options struct {
	// Enabled controls whether enrichment runs. It is opt-in (default false).
	Enabled bool
	// TTL is how long a resolved value stays in the in-memory cache. Zero
	// falls back to a default of 24 hours.
	TTL time.Duration
	// Timeout bounds a single background resolution. Zero falls back to 2s.
	Timeout time.Duration
}

type cacheEntry struct {
	value     Enrichment
	expiresAt time.Time
	pending   bool
}

// Enricher resolves and caches IP enrichment. It is safe for concurrent use.
type Enricher struct {
	opts     Options
	resolver Resolver
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewEnricher returns an Enricher. A nil resolver uses the built-in
// reverse-DNS provider.
func NewEnricher(opts Options, resolver Resolver) *Enricher {
	if resolver == nil {
		resolver = reverseDNSResolver{}
	}
	if opts.TTL <= 0 {
		opts.TTL = 24 * time.Hour
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Second
	}
	return &Enricher{
		opts:     opts,
		resolver: resolver,
		now:      time.Now,
		cache:    make(map[string]cacheEntry),
	}
}

// Lookup returns the enrichment for a client IP passed in string form (for
// example directly from a stored session row). It never blocks on the
// network: a valid cache entry is returned immediately, and a cache miss
// schedules a single background resolution while returning a pending marker.
// When enrichment is disabled, or the address cannot be parsed, it returns a
// zero-value enrichment (Enabled=false).
func (e *Enricher) Lookup(ip string) Enrichment {
	if e == nil || !e.opts.Enabled {
		return Enrichment{}
	}
	addr, ok := normalizeAddress(ip)
	if !ok {
		return Enrichment{}
	}
	if isSkippable(addr) {
		return Enrichment{Enabled: true, Private: true}
	}

	key := addr.String()
	now := e.now()

	e.mu.Lock()
	if entry, exists := e.cache[key]; exists {
		if entry.pending {
			// A background resolution is already in flight; coalesce.
			e.mu.Unlock()
			return Enrichment{Enabled: true, Pending: true}
		}
		if entry.expiresAt.After(now) {
			value := entry.value
			e.mu.Unlock()
			return value
		}
	}
	// Cache miss or expired entry: reserve the key and schedule a single
	// background lookup. Concurrent readers will see the pending marker and
	// coalesce above.
	e.cache[key] = cacheEntry{pending: true}
	e.mu.Unlock()

	go e.resolveAsync(key, addr)
	return Enrichment{Enabled: true, Pending: true}
}

func (e *Enricher) resolveAsync(key string, addr netip.Addr) {
	ctx, cancel := context.WithTimeout(context.Background(), e.opts.Timeout)
	defer cancel()

	value, err := e.resolver.Resolve(ctx, addr)
	if err != nil {
		value = Enrichment{}
	}
	value.Enabled = true
	value.Pending = false

	e.mu.Lock()
	e.cache[key] = cacheEntry{
		value:     value,
		expiresAt: e.now().Add(e.opts.TTL),
	}
	e.mu.Unlock()
}

// reverseDNSResolver resolves the PTR record for a public address.
type reverseDNSResolver struct{}

func (reverseDNSResolver) Resolve(ctx context.Context, addr netip.Addr) (Enrichment, error) {
	names, err := net.DefaultResolver.LookupAddr(ctx, addr.String())
	if err != nil {
		return Enrichment{}, err
	}
	for _, name := range names {
		if host := strings.TrimSuffix(strings.TrimSpace(name), "."); host != "" {
			return Enrichment{Hostname: host}, nil
		}
	}
	return Enrichment{}, nil
}

// normalizeAddress parses and canonicalizes a client IP string. Mapped IPv6
// (::ffff:a.b.c.d) is unmapped to the plain IPv4 address.
func normalizeAddress(ip string) (netip.Addr, bool) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		// Tolerate an "ip:port" form (e.g. from a mis-parsed header value).
		host, _, splitErr := net.SplitHostPort(ip)
		if splitErr != nil {
			return netip.Addr{}, false
		}
		addr, err = netip.ParseAddr(host)
		if err != nil {
			return netip.Addr{}, false
		}
	}
	return addr.Unmap(), true
}

// reservedPrefixes lists address ranges that are not globally routable public
// addresses but that netip does not otherwise flag. They are treated as
// private/skippable so internal infrastructure addresses are never resolved.
var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),      // CGNAT shared address space
	netip.MustParsePrefix("192.0.0.0/24"),       // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),       // documentation (TEST-NET-1)
	netip.MustParsePrefix("198.51.100.0/24"),    // documentation (TEST-NET-2)
	netip.MustParsePrefix("203.0.113.0/24"),     // documentation (TEST-NET-3)
	netip.MustParsePrefix("198.18.0.0/15"),      // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),        // reserved for future use
	netip.MustParsePrefix("255.255.255.255/32"), // limited broadcast
	netip.MustParsePrefix("2001:db8::/32"),      // IPv6 documentation
}

// isSkippable reports whether an address must be classified as private/local
// and never handed to the resolver.
func isSkippable(addr netip.Addr) bool {
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() ||
		!addr.IsGlobalUnicast() {
		return true
	}
	for _, prefix := range reservedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
