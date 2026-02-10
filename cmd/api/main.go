package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/quyxishi/whitebox"
	"github.com/quyxishi/whitebox/internal/api"
	"github.com/quyxishi/whitebox/internal/config"
	mlog "github.com/quyxishi/whitebox/internal/log"
)

func gracefulShutdown(apiServer *http.Server, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	slog.Info("Shutting down gracefully, press Ctrl+C again to force")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "err", err.Error())
	}

	slog.Info("Server exiting")

	// Notify the main goroutine that the shutdown is complete.
	done <- true
}

func hotReloadLoop(cli *CLI, configWrapper *config.WhiteboxConfigWrapper) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)

	for {
		<-sigs // Listen for the sighup signal.
		slog.Info("SIGHUP received, reloading config")

		newConfig, err := cli.LoadConfig()
		if err != nil {
			slog.Error("Unable to reload config file", "err", err)
			continue
		}

		configWrapper.Update(newConfig)
		slog.Info("Config updated successfully")
	}
}

func main() {
	var cli CLI

	parser := kong.Must(&cli,
		kong.Name("whitebox"),
		kong.Description("A Prometheus exporter that provides availability monitoring of external VPN services"),
		kong.Vars{
			"version": whitebox.Version(),
		},
	)

	_, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(fmt.Errorf("unable to parse cli args due: %v", err))
	}

	cfg, err := cli.LoadConfig()
	if err != nil {
		parser.FatalIfErrorf(fmt.Errorf("unable to load config file due: %v", err))
	}

	wrapper := config.NewConfigWrapper(cfg)

	// Initialize the default structured logger writing to stdout
	// This configuration is flexible and can be adapted - e.g, by switching to JSON -
	// to ensure compatibility with log ingestion and aggregation systems.
	opts := mlog.ModuleHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler := mlog.NewModuleHandler(os.Stdout, &opts)
	slog.SetDefault(slog.New(handler))

	if cli.ConfigPath == "" {
		slog.Info("using default configuration (no config path provided)")
	} else {
		slog.Info("using configuration file w/", slog.Int("scopes", len(cfg.Scopes)), slog.String("path", cli.ConfigPath))
	}

	// *

	server := api.NewServer(wrapper)

	// Create a done channel to signal when the shutdown is complete.
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine.
	go gracefulShutdown(server, done)
	go hotReloadLoop(&cli, wrapper)

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("http server error: %s", err))
	}

	// Wait for the graceful shutdown to complete.
	<-done
	slog.Info("Graceful shutdown complete.")
}
