package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func ExampleUsage() {
	loggerConfig := &SetupLoggerConfig{
		Level:                  LogLevelInfo,
		Component:              "auto-setup",
		EnableJSON:             false, // Human-readable for user experience
		EnableUserMessages:     true,
		EnableProgressTracking: true,
		EnableMetrics:          true,
		VerboseMode:            false,
		QuietMode:              false,
		SessionID:              generateSessionID(),
	}

	logger := NewSetupLogger(loggerConfig)
	defer logger.LogSummary()

	errorHandler := NewErrorHandler(logger)

	ctx := &ErrorContext{
		Operation:   "install-go-runtime",
		Component:   "go",
		StartTime:   time.Now(),
		Metadata:    map[string]interface{}{"version": "1.21.0"},
		Logger:      logger,
		SessionID:   loggerConfig.SessionID,
		UserContext: map[string]string{"user_home": os.Getenv("HOME")},
	}

	logger.UserInfo("Starting Go runtime installation")

	if err := exampleInstallRuntime(ctx, errorHandler); err != nil {
		if handleErr := errorHandler.HandleError(ctx, err); handleErr != nil {
		}
		return
	}

	requirements := map[string]interface{}{
		"disk_space":  int64(1024 * 1024 * 1024), // 1GB
		"permissions": []string{"/usr/local", "/opt"},
		"commands":    []string{"git", "curl"},
		"network":     []string{"https://golang.org", "https://github.com"},
	}

	if errors := errorHandler.ValidateEnvironment(ctx, requirements); len(errors) > 0 {
		logger.UserError(fmt.Sprintf("Environment validation failed with %d issues", len(errors)))
		for _, err := range errors {
			if handleErr := errorHandler.HandleError(ctx, err); handleErr != nil {
			}
		}
		return
	}

	cmd := exec.Command("go", "version")
	if err := errorHandler.WrapCommand(ctx, cmd); err != nil {
		if handleErr := errorHandler.HandleError(ctx, err); handleErr != nil {
		}
		return
	}

	logger.UserSuccess("Go runtime installation completed successfully")
}

func exampleInstallRuntime(ctx *ErrorContext, handler *ErrorHandler) error {
	progress := NewProgressInfo("install-go", 100)

	ctx.Logger.WithStage("download").WithProgress(progress).Info("Downloading Go binary")

	downloadURL := "https://golang.org/dl/go1.21.0.linux-amd64.tar.gz"
	targetPath := "/tmp/go1.21.0.linux-amd64.tar.gz"

	for i := int64(0); i <= 30; i++ {
		progress.Update(i, fmt.Sprintf("go1.21.0.linux-amd64.tar.gz (%d%%)", i*100/30))
		time.Sleep(10 * time.Millisecond) // Simulate download time

		if i%10 == 0 {
			ctx.Logger.UserProgress("Downloading Go binary", progress)
		}
	}

	downloadFunc := func() error {
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			return fmt.Errorf("network error: download from %s failed: connection timeout", downloadURL)
		}
		return nil
	}

	if err := handler.RetryOperation(context.Background(), downloadFunc, ErrCodeDownloadFailed, ctx.Logger); err != nil {
		return WrapWithContext(err, "download", "go-binary", map[string]interface{}{
			"url":         downloadURL,
			"target_path": targetPath,
		})
	}

	progress.Update(30, "Download completed")
	ctx.Logger.UserSuccess("Go binary downloaded successfully")

	ctx.Logger.WithStage("extract").Info("Extracting Go binary")

	extractFunc := func() error {
		for i := int64(31); i <= 60; i++ {
			progress.Update(i, fmt.Sprintf("Extracting files (%d%%)", (i-30)*100/30))
			time.Sleep(5 * time.Millisecond)

			if i%10 == 0 {
				ctx.Logger.UserProgress("Extracting Go binary", progress)
			}
		}
		return nil
	}

	if err := extractFunc(); err != nil {
		return NewInstallationError(ErrCodeServerInstallFailed, "go", fmt.Sprintf("Failed to extract Go archive: %v", err))
	}

	progress.Update(60, "Extraction completed")
	ctx.Logger.UserSuccess("Go binary extracted successfully")

	ctx.Logger.WithStage("install").Info("Installing Go to system location")

	installPath := "/usr/local/go"

	installFunc := func() error {
		if err := ValidateWritePermissions(filepath.Dir(installPath)); err != nil {
			return err
		}

		for i := int64(61); i <= 90; i++ {
			progress.Update(i, fmt.Sprintf("Installing to %s (%d%%)", installPath, (i-60)*100/30))
			time.Sleep(5 * time.Millisecond)

			if i%10 == 0 {
				ctx.Logger.UserProgress("Installing Go", progress)
			}
		}

		return nil
	}

	if err := installFunc(); err != nil {
		return WrapWithContext(err, "install", "go-binary", map[string]interface{}{
			"install_path": installPath,
			"source_path":  targetPath,
		})
	}

	ctx.Logger.WithStage("verify").Info("Verifying Go installation")

	verifyFunc := func() error {
		for i := int64(91); i <= 100; i++ {
			progress.Update(i, "Verifying installation")
			time.Sleep(10 * time.Millisecond)

			if i%5 == 0 {
				ctx.Logger.UserProgress("Verifying Go installation", progress)
			}
		}

		cmd := exec.Command("go", "version")
		if err := handler.WrapCommand(ctx, cmd); err != nil {
			return err
		}

		return nil
	}

	if err := verifyFunc(); err != nil {
		return NewValidationError(ErrCodeVersionValidationFailed, "go_version", "", "Go binary not responding correctly")
	}

	progress.Update(100, "Installation completed")
	ctx.Logger.UserSuccess("Go installation verified successfully")

	ctx.Logger.UpdateMetrics(map[string]interface{}{
		"runtimes_installed": 1,
		"bytes_downloaded":   int64(150 * 1024 * 1024), // 150MB
		"files_processed":    1,
	})

	return nil
}

func ExampleErrorScenarios() {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:              LogLevelDebug,
		Component:          "error-demo",
		EnableUserMessages: true,
		VerboseMode:        true,
	})

	errorHandler := NewErrorHandler(logger)

	logger.UserInfo("=== Demonstrating Permission Error ===")
	permErr := fmt.Errorf("insufficient permissions: cannot write to /root/sensitive-file")
	ctx := &ErrorContext{
		Operation: "file-write",
		Component: "file-system",
		Logger:    logger,
	}
	if err := errorHandler.HandleError(ctx, permErr); err != nil {
	}

	logger.UserInfo("\n=== Demonstrating Network Error with Retry ===")
	networkErr := fmt.Errorf("network error: download from https://example.com/file.tar.gz failed: connection timeout after 30s")
	ctx.Operation = "download-file"
	if err := errorHandler.HandleError(ctx, networkErr); err != nil {
	}

	logger.UserInfo("\n=== Demonstrating Command Not Found Error ===")
	cmdErr := NewCommandNotFoundError("nonexistent-command")
	ctx.Operation = "execute-command"
	if err := errorHandler.HandleError(ctx, cmdErr); err != nil {
	}

	logger.UserInfo("\n=== Demonstrating Validation Error ===")
	validationErr := NewConfigurationError("config.yaml", "server.port", "invalid", "must be a number between 1 and 65535", nil)
	ctx.Operation = "config-validation"
	if err := errorHandler.HandleError(ctx, validationErr); err != nil {
	}

	logger.UserInfo("\n=== Demonstrating Installation Error with Alternatives ===")
	installErr := fmt.Errorf("runtime not found: python (version: 3.9+) on platform: linux")
	ctx.Operation = "install-runtime"
	if err := errorHandler.HandleError(ctx, installErr); err != nil {
	}
}

func ExampleProgressTracking() {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:                  LogLevelInfo,
		Component:              "progress-demo",
		EnableUserMessages:     true,
		EnableProgressTracking: true,
	})

	logger.UserInfo("=== Demonstrating Progress Tracking ===")

	progress := NewProgressInfo("process-files", 1000)

	for i := int64(0); i <= 1000; i += 50 {
		fileName := fmt.Sprintf("file_%03d.txt", i)
		progress.Update(i, fileName)

		logger.WithProgress(progress).Debug(fmt.Sprintf("Processing %s", fileName))
		logger.UserProgress("Processing files", progress)

		time.Sleep(100 * time.Millisecond)
	}

	logger.UserSuccess("File processing completed")

	logger.UserInfo("\n=== Demonstrating ETA Calculation ===")
	longProgress := NewProgressInfo("long-operation", 10000)

	for i := int64(0); i <= 10000; i += 1000 {
		longProgress.Update(i, fmt.Sprintf("Step %d", i/1000+1))

		eta := longProgress.GetETA()
		if eta > 0 {
			logger.UserInfo(fmt.Sprintf("Progress: %.1f%%, ETA: %v",
				longProgress.Percentage, eta.Round(time.Second)))
		} else {
			logger.UserInfo(fmt.Sprintf("Progress: %.1f%%", longProgress.Percentage))
		}

		time.Sleep(200 * time.Millisecond)
	}

	logger.UserSuccess("Long operation completed")
}

func ExampleMetricsTracking() {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:         LogLevelInfo,
		Component:     "metrics-demo",
		EnableMetrics: true,
	})
	defer logger.LogSummary()

	logger.UserInfo("=== Demonstrating Metrics Tracking ===")

	operations := []struct {
		name     string
		errors   int
		warnings int
		files    int
		bytes    int64
	}{
		{"download-go", 0, 1, 1, 150 * 1024 * 1024},
		{"extract-archive", 1, 0, 245, 300 * 1024 * 1024},
		{"install-binary", 0, 0, 1, 50 * 1024 * 1024},
		{"verify-installation", 0, 2, 0, 0},
	}

	for _, op := range operations {
		logger.WithOperation(op.name).Info(fmt.Sprintf("Starting %s", op.name))

		for i := 0; i < op.errors; i++ {
			logger.Error("Simulated error occurred")
		}
		for i := 0; i < op.warnings; i++ {
			logger.Warn("Simulated warning issued")
		}

		logger.UpdateMetrics(map[string]interface{}{
			"files_processed":  op.files,
			"bytes_downloaded": op.bytes,
			"operations_count": 1,
		})

		logger.WithOperation(op.name).Info(fmt.Sprintf("Completed %s", op.name))
		time.Sleep(100 * time.Millisecond)
	}

	metrics := logger.GetMetrics()
	logger.UserInfo(fmt.Sprintf("Final metrics: %d errors, %d warnings, %d files processed",
		metrics.ErrorsEncountered, metrics.WarningsIssued, metrics.FilesProcessed))
}

func ExampleIntegrationWithExistingCode() {

	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:              LogLevelInfo,
		Component:          "integration-demo",
		EnableUserMessages: true,
		SessionID:          "demo-session-001",
	})

	errorHandler := NewErrorHandler(logger)

	logger.LogOperationStart("setup-lsp-servers", map[string]interface{}{
		"requested_servers": []string{"gopls", "pylsp", "typescript-language-server"},
		"auto_install":      true,
	})

	servers := []string{"gopls", "pylsp", "typescript-language-server"}
	for i, server := range servers {
		serverCtx := &ErrorContext{
			Operation: fmt.Sprintf("install-%s", server),
			Component: "server-installer",
			StartTime: time.Now(),
			Logger:    logger.WithField("server", server),
		}

		if err := simulateServerInstallation(server, serverCtx, errorHandler); err != nil {
			if handleErr := errorHandler.HandleError(serverCtx, err); handleErr != nil {
			}
			continue
		}

		logger.UserSuccess(fmt.Sprintf("Installed %s (%d/%d)", server, i+1, len(servers)))
	}

	logger.LogOperationComplete("setup-lsp-servers", true)
}

func simulateServerInstallation(server string, ctx *ErrorContext, handler *ErrorHandler) error {
	switch server {
	case "gopls":
		ctx.Logger.WithStage("download").Info("Downloading gopls")
		ctx.Logger.WithStage("install").Info("Installing gopls")
		return nil

	case "pylsp":
		ctx.Logger.Warn("Python LSP server has known compatibility issues")
		return nil

	case "typescript-language-server":
		return NewInstallationError(ErrCodeServerInstallFailed, server, "npm registry unreachable")
	}

	return nil
}
