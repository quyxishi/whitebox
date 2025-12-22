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

	"github.com/quyxishi/whitebox/internal/api"
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

func main() {
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

	// *

	server := api.NewServer()

	// Create a done channel to signal when the shutdown is complete.
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine.
	go gracefulShutdown(server, done)

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("http server error: %s", err))
	}

	// Wait for the graceful shutdown to complete.
	<-done
	slog.Info("Graceful shutdown complete.")
}
