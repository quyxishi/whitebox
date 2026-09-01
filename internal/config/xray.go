package config

import (
	"fmt"
	"time"

	"github.com/quyxishi/whitebox/internal/xraypool"
)

type XrayRecord struct {
	InstanceCache InstanceCacheRecord `yaml:"instance_cache,omitempty"`
}

func NewXrayRecord() XrayRecord {
	return XrayRecord{InstanceCache: NewInstanceCacheRecord()}
}

// InstanceCacheRecord configures reuse of xray instances across probes
type InstanceCacheRecord struct {
	// Pointer so that `enabled: false` is distinguishable from an absent key,
	// which Normalize would otherwise silently turn back on
	Enabled *bool `yaml:"enabled,omitempty"`

	// Idle time after which an unused instance is closed. Fallbacks to 10m
	TTL time.Duration `yaml:"ttl,omitempty"`

	// Maximum cached instances. Fallbacks to 64.
	//
	// Kept modest on purpose: a cached wireguard/amneziawg instance owns a live
	// gVisor netstack, so a large cap trades one memory problem for another
	MaxEntries int `yaml:"max_entries,omitempty"`
}

func NewInstanceCacheRecord() InstanceCacheRecord {
	enabled := true

	return InstanceCacheRecord{
		Enabled:    &enabled,
		TTL:        xraypool.DefaultTTL,
		MaxEntries: xraypool.DefaultMaxEntries,
	}
}

// Normalize applies defaults to fields the yaml omitted
func (c *InstanceCacheRecord) Normalize() {
	if c.Enabled == nil {
		enabled := true
		c.Enabled = &enabled
	}
	if c.TTL <= 0 {
		c.TTL = xraypool.DefaultTTL
	}
	if c.MaxEntries <= 0 {
		c.MaxEntries = xraypool.DefaultMaxEntries
	}
}

// Validate ensures the instance cache configuration semantic correctness
func (c *InstanceCacheRecord) Validate() error {
	if c.TTL < time.Second {
		return fmt.Errorf("xray.instance_cache.ttl must be at least 1s, got %s", c.TTL)
	}
	if c.MaxEntries < 1 {
		return fmt.Errorf("xray.instance_cache.max_entries must be at least 1, got %d", c.MaxEntries)
	}
	return nil
}

// PoolOptions projects the record onto the pool's own options
func (c *InstanceCacheRecord) PoolOptions() xraypool.Options {
	enabled := c.Enabled == nil || *c.Enabled

	return xraypool.Options{
		Enabled:    enabled,
		TTL:        c.TTL,
		MaxEntries: c.MaxEntries,
	}
}
