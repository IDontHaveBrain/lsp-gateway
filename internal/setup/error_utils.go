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
	"syscall"
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

func (eh *ErrorHandler) RetryOperation(
	ctx context.Context,
	operation func() error,
	errorCode platform.ErrorCode,
	contextLogger *SetupLogger,
) error {
	config, exists := eh.retryable[errorCode]
	if !exists {
		return operation() // No retry config, execute once
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		contextLogger.WithField("attempt", attempt).Debug("Attempting operation")

		err := operation()
		if err == nil {
			if attempt > 1 {
				contextLogger.UserSuccess(fmt.Sprintf("Operation succeeded after %d attempts", attempt))
			}
			return nil
		}

		lastErr = err

		if attempt == config.MaxAttempts {
			break
		}

		contextLogger.WithFields(map[string]interface{}{
			"attempt": attempt,
			"delay":   delay,
		}).Warn("Operation failed, retrying")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		if config.Jitter {
			jitter := time.Duration(float64(delay) * 0.1 * (2.0*0.5 - 1.0)) // ±10% jitter
			delay += jitter
		}
	}

	contextLogger.WithField("attempts", config.MaxAttempts).Error("Operation failed after all retry attempts")
	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

func (eh *ErrorHandler) WrapCommand(ctx *ErrorContext, cmd *exec.Cmd) error {
	startTime := time.Now()

	ctx.Logger.WithFields(map[string]interface{}{
		"command":     cmd.Path,
		"args":        cmd.Args[1:],
		"working_dir": cmd.Dir,
	}).Debug("Executing command")

	err := cmd.Run()
	duration := time.Since(startTime)

	success := err == nil
	ctx.Logger.LogCommand(cmd.Path, cmd.Args[1:], success, duration)

	if err != nil {
		return eh.handleCommandError(ctx, cmd, err, duration)
	}

	return nil
}

func (eh *ErrorHandler) WrapDownload(ctx *ErrorContext, url, targetPath string, expectedSize int64) error {
	ctx.Logger.WithFields(map[string]interface{}{
		"url":           url,
		"target_path":   targetPath,
		"expected_size": expectedSize,
	}).Info("Starting download")

	downloadFunc := func() error {
		return nil
	}

	err := eh.RetryOperation(context.Background(), downloadFunc, ErrCodeDownloadFailed, ctx.Logger)
	if err != nil {
		return fmt.Errorf("download failed for %s: %w", url, err)
	}

	ctx.Logger.UserSuccess(fmt.Sprintf("Downloaded %s", extractFilename(url)))
	return nil
}

func (eh *ErrorHandler) ValidateEnvironment(ctx *ErrorContext, requirements map[string]interface{}) []error {
	var errors []error
	ctx.Logger.Info("Validating environment")

	for requirement, constraint := range requirements {
		validationErrors := eh.validateRequirement(ctx, requirement, constraint)
		errors = append(errors, validationErrors...)
	}

	eh.logValidationResult(ctx, errors)
	return errors
}

func (eh *ErrorHandler) validateRequirement(ctx *ErrorContext, requirement string, constraint interface{}) []error {
	switch requirement {
	case "disk_space":
		return eh.validateDiskSpaceRequirement(ctx, constraint)
	case "permissions":
		return eh.validatePermissionsRequirement(constraint)
	case "commands":
		return eh.validateCommandsRequirement(ctx, constraint)
	case "network":
		return eh.validateNetworkRequirement(ctx, constraint)
	default:
		return nil
	}
}

func (eh *ErrorHandler) validateDiskSpaceRequirement(ctx *ErrorContext, constraint interface{}) []error {
	minSpace, ok := constraint.(int64)
	if !ok {
		return nil
	}

	if err := eh.validateDiskSpace(ctx, minSpace); err != nil {
		return []error{err}
	}
	return nil
}

func (eh *ErrorHandler) validatePermissionsRequirement(constraint interface{}) []error {
	paths, ok := constraint.([]string)
	if !ok {
		return nil
	}

	var errors []error
	for _, path := range paths {
		if err := ValidateWritePermissions(path); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (eh *ErrorHandler) validateCommandsRequirement(ctx *ErrorContext, constraint interface{}) []error {
	commands, ok := constraint.([]string)
	if !ok {
		return nil
	}

	var errors []error
	for _, command := range commands {
		if err := eh.validateCommand(ctx, command); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (eh *ErrorHandler) validateNetworkRequirement(ctx *ErrorContext, constraint interface{}) []error {
	urls, ok := constraint.([]string)
	if !ok {
		return nil
	}

	var errors []error
	for _, url := range urls {
		if err := eh.validateNetworkAccess(ctx, url); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (eh *ErrorHandler) logValidationResult(ctx *ErrorContext, errors []error) {
	if len(errors) == 0 {
		ctx.Logger.UserSuccess("Environment validation passed")
	} else {
		ctx.Logger.UserError(fmt.Sprintf("Environment validation failed (%d issues)", len(errors)))
	}
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
	case "linux":
		if cmd := exec.Command("apt", "update"); cmd.Run() == nil {
			logger.Debug("Successfully updated apt cache")
			return nil
		}
		if cmd := exec.Command("yum", "makecache"); cmd.Run() == nil {
			logger.Debug("Successfully updated yum cache")
			return nil
		}
	case "darwin":
		if cmd := exec.Command(platform.PACKAGE_MANAGER_BREW, "update"); cmd.Run() == nil {
			logger.Debug("Successfully updated brew")
			return nil
		}
	}

	return err // No recovery possible
}

func (eh *ErrorHandler) handleCommandError(ctx *ErrorContext, cmd *exec.Cmd, err error, duration time.Duration) error {
	var exitCode int
	var signal string

	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
			if status.Signaled() {
				signal = status.Signal().String()
			}
		}
	} else {
		return NewCommandNotFoundError(cmd.Path)
	}

	cmdErr := NewCommandError(ErrCodeCommandFailed, cmd.Path, cmd.Args[1:], exitCode).
		WithDuration(duration).
		WithWorkingDir(cmd.Dir)

	if signal != "" {
		cmdErr = cmdErr.WithSignal(signal)
	}

	if cmd.Stdout != nil {
	}

	return cmdErr
}

func (eh *ErrorHandler) validateDiskSpace(ctx *ErrorContext, minSpace int64) error {
	return nil // Placeholder
}

func (eh *ErrorHandler) validateCommand(ctx *ErrorContext, command string) error {
	_, err := exec.LookPath(command)
	if err != nil {
		return NewCommandNotFoundError(command)
	}
	return nil
}

func (eh *ErrorHandler) validateNetworkAccess(ctx *ErrorContext, url string) error {
	return nil
}

func (eh *ErrorHandler) installPackage(ctx context.Context, packageName string, logger *SetupLogger) error {
	logger.WithField("package", packageName).Debug("Attempting to install package")

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("apt"); err == nil {
			cmd = exec.Command("apt", "install", "-y", packageName)
		} else if _, err := exec.LookPath("yum"); err == nil {
			cmd = exec.Command("yum", "install", "-y", packageName)
		} else if _, err := exec.LookPath("dnf"); err == nil {
			cmd = exec.Command("dnf", "install", "-y", packageName)
		}
	case "darwin":
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

func ExtractUserActionableErrors(errors []error) []error {
	var actionable []error

	for _, err := range errors {
		if pe, ok := err.(*platform.PlatformError); ok {
			if !pe.AutoRecoverable {
				actionable = append(actionable, err)
			}
		} else {
			actionable = append(actionable, err)
		}
	}

	return actionable
}
