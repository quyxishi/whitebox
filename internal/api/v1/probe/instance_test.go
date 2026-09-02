package probe

import (
	"context"
	"strings"
	"testing"

	"github.com/quyxishi/whitebox/internal/serial"
	"github.com/quyxishi/whitebox/internal/xraypool"
)

const (
	uriWireguard = "wireguard://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzN0emxYS2FJNGY4NnEyOFYzbnhGS2YxcmNoYWt4bWdBbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKCiMgLTEKW1BlZXJdClB1YmxpY0tleSA9IHk2MTdkQ2dNM1g2bEtEanBkdDVhR2NBWmROWW5OT0FwMFMyanFUbGpmZzA9CkFsbG93ZWRJUHMgPSAwLjAuMC4wLzAsIDo6LzAKRW5kcG9pbnQgPSAxLjIuMy40OjI3Nzg5"
	uriAmneziaWG = "awg://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzN0emxYS2FJNGY4NnEyOFYzbnhGS2YzcmNoYWt4bWdCbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKSmMgPSAzCkptaW4gPSA1MApKbWF4ID0gMTAwMApTMSA9IDIwClMyID0gNzgKSDEgPSAzOTEzMTI3OApIMiA9IDgzMjEzODE4NQpIMyA9IDE0MzY5NTc4NTcKSDQgPSAxNjM1ODc3NzQ2CgpbUGVlcl0KUHVibGljS2V5ID0geTYxN2RDZ00zWDZsS0RqcGR0NWFHY0FaZE5Zbk5PQXAwUzNqYVRsamZnMD0KQWxsb3dlZElQcyA9IDAuMC4wLjAvMCwgOjovMApFbmRwb2ludCA9IDEuMi4zLjQ6Mjc3ODkK"
	uriVless     = "vless://" + e2eClientUUID + "@1.2.3.4:443?type=tcp&encryption=none&security=none#poolable"
)

// TestPoolableFromURI is the regression test for the tunnel that died on the
// second probe: a cached wireguard instance keeps dialing through a context
// that was cancelled when the probe which built it returned
func TestPoolableFromURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{"wireguard", uriWireguard, false},
		{"amneziawg", uriAmneziaWG, false},
		{"vless", uriVless, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf, err := serial.ParseURI(serial.CONFIG_BACKEND_XRAYCORE, tt.uri, &serial.ParseParams{})
			if err != nil {
				t.Fatalf("parse uri: %v", err)
			}

			if got := poolable(conf); got != tt.want {
				t.Errorf("poolable = %v, want %v, conf:\n%s", got, tt.want, conf)
			}
		})
	}
}

// TestPoolableFromRawConfig asserts the check reads the generated config rather
// than the ctx uri scheme, so a wireguard outbound that arrived inside a json
// subscription is caught as well
func TestPoolableFromRawConfig(t *testing.T) {
	tests := []struct {
		name string
		conf string
		want bool
	}{
		{
			name: "subscription with a wireguard outbound",
			conf: `{"outbounds":[{"tag":"proxy","protocol":"wireguard"},{"tag":"direct","protocol":"freedom"}]}`,
			want: false,
		},
		{
			name: "wireguard behind a poolable outbound",
			conf: `{"outbounds":[{"tag":"proxy","protocol":"vless"},{"tag":"fallback","protocol":"wireguard"}]}`,
			want: false,
		},
		{
			name: "protocol name is not case sensitive",
			conf: `{"outbounds":[{"tag":"proxy","protocol":"WireGuard"}]}`,
			want: false,
		},
		{
			name: "amneziawg spelled out",
			conf: `{"outbounds":[{"tag":"proxy","protocol":"amneziawg"}]}`,
			want: false,
		},
		{
			name: "no wireguard anywhere",
			conf: `{"outbounds":[{"tag":"proxy","protocol":"hysteria"},{"tag":"direct","protocol":"freedom"}]}`,
			want: true,
		},
		{
			name: "unparseable json",
			conf: `{"outbounds":`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := poolable(tt.conf); got != tt.want {
				t.Errorf("poolable = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAcquireInstanceSkipsCacheForWireguard exercises the real acquire path: a
// wireguard config must build a fresh instance every time and leave nothing
// behind, with the cache enabled
func TestAcquireInstanceSkipsCacheForWireguard(t *testing.T) {
	if testing.Short() {
		t.Skip("stands up a real gVisor netstack per acquire")
	}

	pool := NewInstancePool(xraypool.Options{Enabled: true})
	t.Cleanup(func() { _ = pool.Close() })

	h := &ProbeHandler{pool: pool}

	conf, err := serial.ParseURI(serial.CONFIG_BACKEND_XRAYCORE, uriWireguard, &serial.ParseParams{})
	if err != nil {
		t.Fatalf("parse uri: %v", err)
	}
	if !strings.Contains(conf, `"protocol":"wireguard"`) {
		t.Fatalf("generated config is not a wireguard outbound:\n%s", conf)
	}

	const acquires = 3
	for i := range acquires {
		lease, err := h.acquireInstance(context.Background(), conf)
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		if lease.Cached() {
			t.Errorf("acquire %d reported a cache hit", i)
		}
		lease.Release()
	}

	s := pool.Stats()
	if s.Size != 0 {
		t.Errorf("Size = %d, want 0", s.Size)
	}
	if s.Hits != 0 {
		t.Errorf("Hits = %d, want 0", s.Hits)
	}
	if s.Misses != acquires {
		t.Errorf("Misses = %d, want %d", s.Misses, acquires)
	}
}
