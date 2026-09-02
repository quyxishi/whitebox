package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/quyxishi/whitebox/internal/xraypool"

	"github.com/xtls/xray-core/core"
)

var (
	ErrLoadConfig    = errors.New("unable to load xray config")
	ErrNewInstance   = errors.New("unable to init xray instance")
	ErrStartInstance = errors.New("unable to start xray instance")
)

// InstancePool caches started xray instances across probes
type InstancePool = xraypool.Pool[*core.Instance]

// NewInstancePool builds the pool that ProbeHandler leases instances from
func NewInstancePool(opts xraypool.Options) *InstancePool {
	return xraypool.New[*core.Instance](opts)
}

// instanceKey identifies a generated xray config.
//
// The config json is hashed rather than kept verbatim because it carries client
// uuids and wireguard private keys, and the key reaches debug logs
func instanceKey(conf string) string {
	sum := sha256.Sum256([]byte(conf))
	return hex.EncodeToString(sum[:])
}

// newXrayInstance loads, constructs and starts an instance, closing whatever was
// already built if any step fails.
//
// The close-on-failed-start is not incidental: for wireguard and amneziawg
// outbounds the gVisor netstack is stood up inside core.New, so returning from a
// failed Start without closing orphans a whole netstack
func newXrayInstance(conf string) (*core.Instance, error) {
	xrayConf, err := core.LoadConfig("json", strings.NewReader(conf))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLoadConfig, err)
	}

	instance, err := core.New(xrayConf)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNewInstance, err)
	}

	if err := instance.Start(); err != nil {
		if cerr := instance.Close(); cerr != nil {
			slog.Error("failed to close xray instance after failed start", "err", cerr)
		}
		return nil, fmt.Errorf("%w: %v", ErrStartInstance, err)
	}

	return instance, nil
}

// wireguardProtocols are the xray-core outbound protocol names that stand up a
// wireguard tunnel. The awg scheme is serialized as "wireguard" too, its
// obfuscation parameters riding along in the settings; the other spellings are
// listed because a json subscription writes the protocol name itself
var wireguardProtocols = map[string]struct{}{
	"wireguard": {},
	"amneziawg": {},
	"awg":       {},
}

func poolable(conf string) bool {
	var parsed struct {
		Outbounds []struct {
			Protocol string `json:"protocol"`
		} `json:"outbounds"`
	}

	if err := json.Unmarshal([]byte(conf), &parsed); err != nil {
		// unreadable json is about to fail in core.LoadConfig anyway, and there
		// is nothing worth caching behind it
		return false
	}

	for _, out := range parsed.Outbounds {
		if _, found := wireguardProtocols[strings.ToLower(strings.TrimSpace(out.Protocol))]; found {
			return false
		}
	}

	return true
}

// acquireInstance leases a started instance for the given generated config
func (h *ProbeHandler) acquireInstance(ctx context.Context, conf string) (*xraypool.Lease[*core.Instance], error) {
	build := func(context.Context) (*core.Instance, error) {
		return newXrayInstance(conf)
	}

	if !poolable(conf) {
		return h.pool.AcquireUncached(ctx, build)
	}

	return h.pool.Acquire(ctx, instanceKey(conf), build)
}
