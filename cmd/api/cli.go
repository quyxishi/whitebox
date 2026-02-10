package main

import (
	"github.com/alecthomas/kong"
	"github.com/quyxishi/whitebox/internal/config"
)

type CLI struct {
	Version    kong.VersionFlag `short:"v" help:"Print version information and exit."`
	ConfigPath string           `name:"config.file" short:"c" help:"Path to whitebox config file."`
}

func (h *CLI) LoadConfig() (*config.WhiteboxConfig, error) {
	// if no config path provided, return default config
	if h.ConfigPath == "" {
		cfg := config.NewWhiteboxConfig()
		return &cfg, nil
	}

	// if path provided, load from file
	return config.Load(h.ConfigPath)
}
