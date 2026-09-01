package main

import (
	"github.com/alecthomas/kong"
	"github.com/quyxishi/whitebox/internal/config"
)

type CLI struct {
	Version    kong.VersionFlag `short:"v" help:"Print version information and exit."`
	ConfigPath string           `name:"config.file" short:"c" help:"Path to whitebox config file."`
	ListenAddr string           `name:"web.listen-address" env:"WHITEBOX_LISTEN_ADDRESS" default:":9116" help:"Address to listen on for the exporter, either [host]:port or a bare port."`
	LogLevel   string           `name:"log.level" short:"l" env:"WHITEBOX_LOG_LEVEL" enum:"debug,info,warn,error" default:"info" help:"Only log messages with the given severity or above. One of: [debug, info, warn, error]."`
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
