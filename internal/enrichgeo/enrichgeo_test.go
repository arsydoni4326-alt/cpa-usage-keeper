package enrichgeo

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

type stubResolver struct {
	result Enrichment
	err    error
	calls  atomic.Int64
}

func (r *stubResolver) Resolve(_ context.Context, _ netip.Addr) (Enrichment, error) {
	r.calls.Add(1)
	return r.result, r.err
}

func TestLookupDisabledReturnsZeroValue(t *testing.T) {
	enricher := NewEnricher(Options{Enabled: false}, nil)
	got := enricher.Lookup("8.8.8.8")
	if got.Enabled {
		t.Fatalf("Lookup returned enabled=%v, want disabled", got.Enabled)
	}
}

func TestLookupInvalidIPDoesNotResolve(t *testing.T) {
	resolver := &stubResolver{result: Enrichment{Hostname: "a.example.com"}}
	enricher := NewEnricher(Options{Enabled: true}, resolver)
	for _, ip := range []string{"", "not-an-ip", "   "} {
		got := enricher.Lookup(ip)
		if got.Enabled {
			t.Fatalf("Lookup(%q) returned enabled=%v, want zero", ip, got.Enabled)
		}
	}
	if resolver.calls.Load() != 0 {
		t.Fatalf("resolver called %d times for invalid IPs, want 0", resolver.calls.Load())
	}
}

func TestLookupPrivateIPIsClassifiedLocallyAndNotResolved(t *testing.T) {
	resolver := &stubResolver{}
	enricher := NewEnricher(Options{Enabled: true}, resolver)
	for _, ip := range []string{
		"127.0.0.1", "10.1.2.3", "192.168.1.1", "172.16.5.5",
		"169.254.10.1", "100.64.0.9", "192.0.2.10", "2001:db8::1",
		"::1", "fc00::1",
	} {
		got := enricher.Lookup(ip)
		if !got.Enabled || !got.Private {
			t.Fatalf("Lookup(%q) = %+v, want Enabled+Private", ip, got)
		}
	}
	if resolver.calls.Load() != 0 {
		t.Fatalf("resolver called %d times, want 0", resolver.calls.Load())
	}
}

func TestLookupPublicIPReturnsPendingThenCachedValue(t *testing.T) {
	resolver := &stubResolver{result: Enrichment{Hostname: "host-a.example.com"}}
	enricher := NewEnricher(Options{Enabled: true, TTL: time.Hour, Timeout: time.Second}, resolver)
	first := enricher.Lookup("8.8.8.8")
	if !first.Enabled || !first.Pending {
		t.Fatalf("first Lookup = %+v, want enabled+pending", first)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		entry := enricher.Lookup("8.8.8.8")
		if !entry.Pending && entry.Hostname == "host-a.example.com" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("resolved value not cached; calls=%d", resolver.calls.Load())
		}
		time.Sleep(time.Millisecond)
	}
	if resolver.calls.Load() != 1 {
		t.Fatalf("resolver called %d times, want 1", resolver.calls.Load())
	}
}

func TestLookupCoalescesConcurrentReads(t *testing.T) {
	resolver := &stubResolver{result: Enrichment{Hostname: "host-b.example.com"}}
	enricher := NewEnricher(Options{Enabled: true, TTL: time.Hour, Timeout: time.Second}, resolver)
	done := make(chan struct{})
	for i := 0; i < 16; i++ {
		go func() {
			_ = enricher.Lookup("9.9.9.9")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 16; i++ {
		<-done
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if entry := enricher.Lookup("9.9.9.9"); !entry.Pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("resolved value not cached; calls=%d", resolver.calls.Load())
		}
		time.Sleep(time.Millisecond)
	}
	if resolver.calls.Load() != 1 {
		t.Fatalf("resolver called %d times for concurrent reads, want 1", resolver.calls.Load())
	}
}

func TestLookupExpiredEntryIsReResolved(t *testing.T) {
	resolver := &stubResolver{result: Enrichment{Hostname: "host-c.example.com"}}
	enricher := NewEnricher(Options{Enabled: true, TTL: 20 * time.Millisecond, Timeout: time.Second}, resolver)
	_ = enricher.Lookup("1.1.1.1")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if entry := enricher.Lookup("1.1.1.1"); !entry.Pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("first resolution not cached")
		}
		time.Sleep(time.Millisecond)
	}
	firstCalls := resolver.calls.Load()
	time.Sleep(30 * time.Millisecond)
	second := enricher.Lookup("1.1.1.1")
	if !second.Pending {
		t.Fatalf("expired entry not re-resolved, got %+v", second)
	}
	// Wait for the background re-resolution goroutine to finish.
	deadline = time.Now().Add(2 * time.Second)
	for {
		if resolver.calls.Load() > firstCalls {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("re-resolution not completed; calls=%d, first=%d", resolver.calls.Load(), firstCalls)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestIsSkippablePublicAndReserved(t *testing.T) {
	if isSkippable(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("8.8.8.8 should not be skippable")
	}
	if !isSkippable(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("127.0.0.1 should be skippable")
	}
	for _, raw := range []string{
		"100.64.0.1", "192.0.0.1", "203.0.113.77", "198.18.0.5",
		"240.0.0.1", "255.255.255.255", "198.51.100.8", "2001:db8::42",
	} {
		addr := netip.MustParseAddr(raw)
		if !isSkippable(addr) {
			t.Fatalf("expected %s to be skippable", raw)
		}
	}
}

func TestNormalizeAddressVariants(t *testing.T) {
	addr, ok := normalizeAddress("::ffff:203.0.113.5")
	if !ok || !addr.Is4() || addr.String() != "203.0.113.5" {
		t.Fatalf("mapped IPv6 = %s, want 203.0.113.5", addr.String())
	}
	addr, ok = normalizeAddress("8.8.8.8:53")
	if !ok || addr.String() != "8.8.8.8" {
		t.Fatalf("ip:port = %s, want 8.8.8.8", addr.String())
	}
}

func TestResolverErrorYieldsEnabledNoHostname(t *testing.T) {
	resolver := &stubResolver{err: errors.New("dns down")}
	enricher := NewEnricher(Options{Enabled: true, TTL: time.Hour, Timeout: time.Second}, resolver)
	_ = enricher.Lookup("4.4.4.4")
	deadline := time.Now().Add(2 * time.Second)
	for {
		entry := enricher.Lookup("4.4.4.4")
		if !entry.Pending {
			if !entry.Enabled || entry.Hostname != "" || entry.Private {
				t.Fatalf("error case: got %+v, want enabled+empty", entry)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("resolution did not settle")
		}
		time.Sleep(time.Millisecond)
	}
}
