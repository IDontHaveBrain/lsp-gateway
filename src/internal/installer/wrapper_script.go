package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WrapperScriptBuilder struct {
	platform       PlatformInfo
	language       string
	executablePath string
	args           []string
	envVars        map[string]string
}

func NewWrapperScriptBuilder(platform PlatformInfo, language string) *WrapperScriptBuilder {
	return &WrapperScriptBuilder{
		platform: platform,
		language: language,
		envVars:  make(map[string]string),
	}
}

func (w *WrapperScriptBuilder) SetExecutable(path string) *WrapperScriptBuilder {
	w.executablePath = path
	return w
}

func (w *WrapperScriptBuilder) SetArgs(args ...string) *WrapperScriptBuilder {
	w.args = args
	return w
}

func (w *WrapperScriptBuilder) SetEnvVar(key, value string) *WrapperScriptBuilder {
	w.envVars[key] = value
	return w
}

func (w *WrapperScriptBuilder) Build() (string, error) {
	if w.executablePath == "" {
		return "", fmt.Errorf("executable path not set")
	}

	if w.platform.IsWindows() {
		return w.buildWindowsWrapper(), nil
	}
	return w.buildUnixWrapper(), nil
}

func (w *WrapperScriptBuilder) WriteToFile(scriptPath string) error {
	content, err := w.Build()
	if err != nil {
		return err
	}

	perm := os.FileMode(0755)
	if w.platform.IsWindows() {
		perm = 0644
	}

	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(scriptPath, []byte(content), perm); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	return nil
}

func (w *WrapperScriptBuilder) buildUnixWrapper() string {
	var sb strings.Builder

	sb.WriteString("#!/bin/sh\n")
	sb.WriteString(fmt.Sprintf("# Wrapper script for %s language server\n\n", w.language))

	// Environment variables
	for key, value := range w.envVars {
		sb.WriteString(fmt.Sprintf("export %s=\"%s\"\n", key, value))
	}
	if len(w.envVars) > 0 {
		sb.WriteString("\n")
	}

	// Determine script directory
	sb.WriteString("# Get script directory\n")
	sb.WriteString("SCRIPT_DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n\n")

	// Execute the actual binary
	sb.WriteString("# Execute the language server\n")
	sb.WriteString(fmt.Sprintf("exec \"$SCRIPT_DIR/%s\"", filepath.Base(w.executablePath)))

	// Add arguments
	if len(w.args) > 0 {
		for _, arg := range w.args {
			sb.WriteString(fmt.Sprintf(" %s", arg))
		}
	}

	// Pass through all command-line arguments
	sb.WriteString(" \"$@\"\n")

	return sb.String()
}

func (w *WrapperScriptBuilder) buildWindowsWrapper() string {
	var sb strings.Builder

	sb.WriteString("@echo off\n")
	sb.WriteString(fmt.Sprintf("REM Wrapper script for %s language server\n\n", w.language))

	// Environment variables
	for key, value := range w.envVars {
		sb.WriteString(fmt.Sprintf("set %s=%s\n", key, value))
	}
	if len(w.envVars) > 0 {
		sb.WriteString("\n")
	}

	// Determine script directory
	sb.WriteString("REM Get script directory\n")
	sb.WriteString("set SCRIPT_DIR=%~dp0\n\n")

	// Execute the actual binary
	sb.WriteString("REM Execute the language server\n")
	exeName := filepath.Base(w.executablePath)
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}
	sb.WriteString(fmt.Sprintf("\"%%SCRIPT_DIR%%\\%s\"", exeName))

	// Add arguments
	if len(w.args) > 0 {
		for _, arg := range w.args {
			sb.WriteString(fmt.Sprintf(" %s", arg))
		}
	}

	// Pass through all command-line arguments
	sb.WriteString(" %*\n")

	return sb.String()
}
