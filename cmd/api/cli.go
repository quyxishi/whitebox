package main

import (
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/goccy/go-yaml"
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
	file, err := os.Open(h.ConfigPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("failed to close file instance", "err", err)
		}
	}()

	var config config.WhiteboxConfig
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
