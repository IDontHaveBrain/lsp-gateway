package setup

import (
	"context"
	"errors"
	"fmt"
	"lsp-gateway/internal/platform"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ErrorHandler struct {
	logger    *SetupLogger
	retryable map[platform.ErrorCode]RetryConfig
	handlers  map[platform.ErrorCode]RecoveryHandler
}

type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Jitter        bool
}

type RecoveryHandler func(ctx context.Context, err error, logger *SetupLogger) error

type ErrorContext struct {
	Operation   string
	Component   string
	StartTime   time.Time
	Metadata    map[string]interface{}
	Logger      *SetupLogger
	Progress    *ProgressInfo
	SessionID   string
	UserContext map[string]string
}

func NewErrorHandler(logger *SetupLogger) *ErrorHandler {
	handler := &ErrorHandler{
		logger:    logger,
		retryable: make(map[platform.ErrorCode]RetryConfig),
		handlers:  make(map[platform.ErrorCode]RecoveryHandler),
	}

	handler.setupDefaultRetryConfigs()
	handler.setupDefaultRecoveryHandlers()

	return handler
}

func (eh *ErrorHandler) HandleError(ctx *ErrorContext, err error) error {
	if err == nil {
		return nil
	}

	contextLogger := eh.logger.WithFields(map[string]interface{}{
		"operation": ctx.Operation,
		"component": ctx.Component,
		"metadata":  ctx.Metadata,
	}).WithError(err)

	if pe, ok := err.(*platform.PlatformError); ok {
		contextLogger = contextLogger.WithField("error_code", pe.Code)

		if handler, exists := eh.handlers[pe.Code]; exists {
			contextLogger.Info("Attempting automatic recovery")
			if recoveryErr := handler(context.Background(), err, contextLogger); recoveryErr == nil {
				contextLogger.UserSuccess("Automatically resolved the issue")
				return nil
			} else {
				contextLogger.WithError(recoveryErr).Warn("Automatic recovery failed")
			}
		}

		guidance := pe.GetUserGuidance()
		eh.provideUserGuidance(contextLogger, guidance)

		return err
	}

	contextLogger.Error("Unhandled error occurred")
	eh.provideGenericGuidance(contextLogger, err)

	return err
}

func (eh *ErrorHandler) setupDefaultRetryConfigs() {
	eh.retryable[ErrCodeDownloadFailed] = RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  2 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}

	eh.retryable[ErrCodeNetworkUnavailable] = RetryConfig{
		MaxAttempts:   2,
		InitialDelay:  5 * time.Second,
		MaxDelay:      20 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}

	eh.retryable[ErrCodeCommandTimeout] = RetryConfig{
		MaxAttempts:   2,
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	eh.retryable[platform.ErrCodePackageManagerFailed] = RetryConfig{
		MaxAttempts:   2,
		InitialDelay:  3 * time.Second,
		MaxDelay:      15 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

func (eh *ErrorHandler) setupDefaultRecoveryHandlers() {
	eh.handlers[platform.ErrCodeInsufficientPermissions] = eh.recoverPermissions

	eh.handlers[ErrCodeCommandNotFound] = eh.recoverMissingCommand

	eh.handlers[platform.ErrCodePackageManagerFailed] = eh.recoverPackageManager
}

func (eh *ErrorHandler) recoverPermissions(ctx context.Context, err error, logger *SetupLogger) error {
	pe, ok := err.(*platform.PlatformError)
	if !ok {
		return err
	}

	logger.Debug("Attempting permission recovery")

	path, exists := pe.SystemInfo["path"].(string)
	if !exists {
		return err
	}

	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		logger.WithError(err).Debug("Failed to create parent directories")
		return err
	}

	logger.Debug("Successfully created parent directories")
	return nil
}

func (eh *ErrorHandler) recoverMissingCommand(ctx context.Context, err error, logger *SetupLogger) error {
	ce, ok := err.(*CommandError)
	if !ok {
		return err
	}

	logger.WithField("command", ce.Command).Debug("Attempting to recover missing command")

	switch ce.Command {
	case "git":
		return eh.installPackage(ctx, "git", logger)
	case "curl":
		return eh.installPackage(ctx, "curl", logger)
	case "wget":
		return eh.installPackage(ctx, "wget", logger)
	}

	return err // No recovery available
}

func (eh *ErrorHandler) recoverPackageManager(ctx context.Context, err error, logger *SetupLogger) error {
	logger.Debug("Attempting package manager recovery")

	switch runtime.GOOS {
	case OS_LINUX:
		if cmd := exec.Command("apt", "update"); cmd.Run() == nil {
			logger.Debug("Successfully updated apt cache")
			return nil
		}
		if cmd := exec.Command("yum", "makecache"); cmd.Run() == nil {
			logger.Debug("Successfully updated yum cache")
			return nil
		}
	case OS_DARWIN:
		if cmd := exec.Command(platform.PACKAGE_MANAGER_BREW, "update"); cmd.Run() == nil {
			logger.Debug("Successfully updated brew")
			return nil
		}
	}

	return err // No recovery possible
}

func (eh *ErrorHandler) installPackage(ctx context.Context, packageName string, logger *SetupLogger) error {
	logger.WithField("package", packageName).Debug("Attempting to install package")

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case OS_LINUX:
		if _, err := exec.LookPath("apt"); err == nil {
			cmd = exec.Command("apt", "install", "-y", packageName)
		} else if _, err := exec.LookPath("yum"); err == nil {
			cmd = exec.Command("yum", "install", "-y", packageName)
		} else if _, err := exec.LookPath("dnf"); err == nil {
			cmd = exec.Command("dnf", "install", "-y", packageName)
		}
	case OS_DARWIN:
		if _, err := exec.LookPath(platform.PACKAGE_MANAGER_BREW); err == nil {
			cmd = exec.Command(platform.PACKAGE_MANAGER_BREW, "install", packageName)
		}
	}

	if cmd == nil {
		return errors.New("no package manager available")
	}

	err := cmd.Run()
	if err != nil {
		logger.WithError(err).WithField("package", packageName).Warn("Failed to install package")
		return err
	}

	logger.WithField("package", packageName).Info("Successfully installed package")
	return nil
}

func (eh *ErrorHandler) provideUserGuidance(logger *SetupLogger, guidance platform.UserGuidance) {
	logger.UserError(guidance.Problem)

	if guidance.Suggestion != "" {
		logger.UserInfo(fmt.Sprintf("Suggestion: %s", guidance.Suggestion))
	}

	if len(guidance.RecoverySteps) > 0 {
		logger.UserInfo("Recovery steps:")
		for i, step := range guidance.RecoverySteps {
			logger.UserInfo(fmt.Sprintf("  %d. %s", i+1, step))
		}
	}

	if len(guidance.RequiredTools) > 0 {
		logger.UserInfo(fmt.Sprintf("Required tools: %s", strings.Join(guidance.RequiredTools, ", ")))
	}

	if guidance.EstimatedTime != "" {
		logger.UserInfo(fmt.Sprintf("Estimated time: %s", guidance.EstimatedTime))
	}

	if guidance.Documentation != "" {
		logger.UserInfo(fmt.Sprintf("Documentation: %s", guidance.Documentation))
	}
}

func (eh *ErrorHandler) provideGenericGuidance(logger *SetupLogger, err error) {
	logger.UserError("An unexpected error occurred")
	logger.UserInfo("Suggestion: Check the logs for more details and try again")

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "permission") {
		logger.UserInfo("This looks like a permission issue. Try running with elevated privileges.")
	} else if strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") {
		logger.UserInfo("This looks like a network issue. Check your internet connection.")
	} else if strings.Contains(errStr, "space") || strings.Contains(errStr, "disk") {
		logger.UserInfo("This looks like a disk space issue. Free up some space and try again.")
	}
}

func WrapWithContext(err error, operation, component string, metadata map[string]interface{}) error {
	if err == nil {
		return nil
	}

	if pe, ok := err.(*platform.PlatformError); ok {
		return pe.WithSystemInfo(map[string]interface{}{
			"operation": operation,
			"component": component,
			"metadata":  metadata,
		})
	}

	return platform.NewPlatformError(
		"GENERIC_ERROR",
		err.Error(),
		fmt.Sprintf("Error in %s (%s)", operation, component),
		"Check the logs and try again",
	).WithSystemInfo(metadata).WithCause(err)
}

func IsTemporaryError(err error) bool {
	if pe, ok := err.(*platform.PlatformError); ok {
		switch pe.Code {
		case ErrCodeNetworkUnavailable, ErrCodeDownloadFailed, ErrCodeCommandTimeout:
			return true
		}
	}

	errStr := strings.ToLower(err.Error())
	temporaryPatterns := []string{
		"timeout", "temporary", "try again", "unavailable", "busy",
		"connection reset", "connection refused",
	}

	for _, pattern := range temporaryPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

func IsCriticalError(err error) bool {
	if pe, ok := err.(*platform.PlatformError); ok {
		return pe.Severity == platform.SeverityCritical
	}
	return false
}
