package cli

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ServerShutdownFunc func() error

type ServerLifecycleConfig struct {
	ServerName      string
	Port            int
	ShutdownFunc    ServerShutdownFunc
	ShutdownTimeout time.Duration
}

func waitForShutdownSignal(ctx context.Context, config *ServerLifecycleConfig, serverErr <-chan error) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	log.Printf("[INFO] %s started successfully on port %d, waiting for requests\n", config.ServerName, config.Port)

	select {
	case <-ctx.Done():
		log.Printf("[INFO] Context cancelled, initiating graceful shutdown\n")
	case <-sigCh:
		log.Printf("[INFO] Received shutdown signal, initiating graceful shutdown\n")
	case err := <-serverErr:
		log.Printf("[ERROR] Server error encountered: %v\n", err)
		return err
	}

	return performGracefulShutdown(config)
}

func performGracefulShutdown(config *ServerLifecycleConfig) error {
	log.Printf("[INFO] Shutting down %s\n", config.ServerName)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer shutdownCancel()

	done := make(chan error, 1)
	go func() {
		done <- config.ShutdownFunc()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[ERROR] Error during %s shutdown: %v\n", config.ServerName, err)
		} else {
			log.Printf("[INFO] %s shutdown completed\n", config.ServerName)
		}
	case <-shutdownCtx.Done():
		log.Printf("[WARN] %s shutdown timed out after %v\n", config.ServerName, config.ShutdownTimeout)
	}

	log.Printf("[INFO] %s stopped successfully\n", config.ServerName)
	return nil
}

func createServerErrorChannel() chan error {
	return make(chan error, 1)
}

func logServerStart(serverName string, port int) {
	log.Printf("[INFO] Starting %s on port %d\n", serverName, port)
}

func logServerError(serverName string, err error) {
	log.Printf("[ERROR] %s error: %v\n", serverName, err)
}
