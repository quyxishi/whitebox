package config

import "sync/atomic"

type WhiteboxConfigWrapper struct {
	ptr atomic.Pointer[WhiteboxConfig]
}

func NewConfigWrapper(initial *WhiteboxConfig) *WhiteboxConfigWrapper {
	c := &WhiteboxConfigWrapper{}
	c.ptr.Store(initial)
	return c
}

func (c *WhiteboxConfigWrapper) Get() *WhiteboxConfig {
	return c.ptr.Load()
}

func (c *WhiteboxConfigWrapper) Update(new *WhiteboxConfig) {
	c.ptr.Store(new)
}
