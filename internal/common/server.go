package common

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ServerLifecycleManager struct {
	shutdownTimeout time.Duration
	errorCh         chan error
}

func NewServerLifecycleManager(shutdownTimeout time.Duration) *ServerLifecycleManager {
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	return &ServerLifecycleManager{
		shutdownTimeout: shutdownTimeout,
		errorCh:         make(chan error, 1),
	}
}

type HTTPServerConfig struct {
	Port         int
	Handler      http.Handler
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (slm *ServerLifecycleManager) RunHTTPServer(ctx context.Context, config HTTPServerConfig) error {
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 30 * time.Second
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(FORMAT_PORT_NUMBER, config.Port),
		Handler:      config.Handler,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slm.errorCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
	case <-sigCh:
	case err := <-slm.errorCh:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), slm.shutdownTimeout)
	defer shutdownCancel()

	return server.Shutdown(shutdownCtx)
}

type ServiceConfig struct {
	StartFunc func() error
	StopFunc  func() error
	Name      string
}

func (slm *ServerLifecycleManager) RunService(ctx context.Context, config ServiceConfig) error {
	go func() {
		if err := config.StartFunc(); err != nil {
			slm.errorCh <- fmt.Errorf("%s service error: %w", config.Name, err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
	case <-sigCh:
	case err := <-slm.errorCh:
		return err
	}

	if config.StopFunc != nil {
		return config.StopFunc()
	}

	return nil
}
