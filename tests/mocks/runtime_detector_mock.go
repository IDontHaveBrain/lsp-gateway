package mocks

import (
	"context"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/setup"
	"time"
)

type MockRuntimeDetector struct {
	DetectGoFunc     func(ctx context.Context) (*setup.RuntimeInfo, error)
	DetectPythonFunc func(ctx context.Context) (*setup.RuntimeInfo, error)
	DetectNodejsFunc func(ctx context.Context) (*setup.RuntimeInfo, error)
	DetectJavaFunc   func(ctx context.Context) (*setup.RuntimeInfo, error)
	DetectAllFunc    func(ctx context.Context) (*setup.DetectionReport, error)
	SetLoggerFunc    func(logger *setup.SetupLogger)
	SetTimeoutFunc   func(timeout time.Duration)

	DetectGoCalls     []context.Context
	DetectPythonCalls []context.Context
	DetectNodejsCalls []context.Context
	DetectJavaCalls   []context.Context
	DetectAllCalls    []context.Context
	SetLoggerCalls    []*setup.SetupLogger
	SetTimeoutCalls   []time.Duration
}

func NewMockRuntimeDetector() *MockRuntimeDetector {
	return &MockRuntimeDetector{
		DetectGoCalls:     make([]context.Context, 0),
		DetectPythonCalls: make([]context.Context, 0),
		DetectNodejsCalls: make([]context.Context, 0),
		DetectJavaCalls:   make([]context.Context, 0),
		DetectAllCalls:    make([]context.Context, 0),
		SetLoggerCalls:    make([]*setup.SetupLogger, 0),
		SetTimeoutCalls:   make([]time.Duration, 0),
	}
}

func (m *MockRuntimeDetector) DetectGo(ctx context.Context) (*setup.RuntimeInfo, error) {
	m.DetectGoCalls = append(m.DetectGoCalls, ctx)
	if m.DetectGoFunc != nil {
		return m.DetectGoFunc(ctx)
	}
	return &setup.RuntimeInfo{
		Name:      "go",
		Installed: true,
		Version:   "1.24.0",
		ParsedVersion: &setup.Version{
			Major:    1,
			Minor:    24,
			Patch:    0,
			Original: "1.24.0",
		},
		Compatible:   true,
		MinVersion:   "1.20.0",
		Path:         "/usr/local/go/bin/go",
		WorkingDir:   "/tmp",
		DetectionCmd: "go version",
		Issues:       []string{},
		Warnings:     []string{},
		Metadata:     map[string]interface{}{"goroot": "/usr/local/go"},
		DetectedAt:   time.Now(),
		Duration:     time.Millisecond * 50,
	}, nil
}

func (m *MockRuntimeDetector) DetectPython(ctx context.Context) (*setup.RuntimeInfo, error) {
	m.DetectPythonCalls = append(m.DetectPythonCalls, ctx)
	if m.DetectPythonFunc != nil {
		return m.DetectPythonFunc(ctx)
	}
	return &setup.RuntimeInfo{
		Name:      "python",
		Installed: true,
		Version:   "3.11.0",
		ParsedVersion: &setup.Version{
			Major:    3,
			Minor:    11,
			Patch:    0,
			Original: "3.11.0",
		},
		Compatible:   true,
		MinVersion:   "3.8.0",
		Path:         "/usr/bin/python3",
		WorkingDir:   "/tmp",
		DetectionCmd: "python3 --version",
		Issues:       []string{},
		Warnings:     []string{},
		Metadata:     map[string]interface{}{"python_path": "/usr/bin/python3"},
		DetectedAt:   time.Now(),
		Duration:     time.Millisecond * 30,
	}, nil
}

func (m *MockRuntimeDetector) DetectNodejs(ctx context.Context) (*setup.RuntimeInfo, error) {
	m.DetectNodejsCalls = append(m.DetectNodejsCalls, ctx)
	if m.DetectNodejsFunc != nil {
		return m.DetectNodejsFunc(ctx)
	}
	return &setup.RuntimeInfo{
		Name:      "nodejs",
		Installed: true,
		Version:   "20.0.0",
		ParsedVersion: &setup.Version{
			Major:    20,
			Minor:    0,
			Patch:    0,
			Original: "20.0.0",
		},
		Compatible:   true,
		MinVersion:   "18.0.0",
		Path:         "/usr/bin/node",
		WorkingDir:   "/tmp",
		DetectionCmd: "node --version",
		Issues:       []string{},
		Warnings:     []string{},
		Metadata:     map[string]interface{}{"npm_version": "10.0.0"},
		DetectedAt:   time.Now(),
		Duration:     time.Millisecond * 40,
	}, nil
}

func (m *MockRuntimeDetector) DetectJava(ctx context.Context) (*setup.RuntimeInfo, error) {
	m.DetectJavaCalls = append(m.DetectJavaCalls, ctx)
	if m.DetectJavaFunc != nil {
		return m.DetectJavaFunc(ctx)
	}
	return &setup.RuntimeInfo{
		Name:      "java",
		Installed: true,
		Version:   "17.0.0",
		ParsedVersion: &setup.Version{
			Major:    17,
			Minor:    0,
			Patch:    0,
			Original: "17.0.0",
		},
		Compatible:   true,
		MinVersion:   "11.0.0",
		Path:         "/usr/bin/java",
		WorkingDir:   "/tmp",
		DetectionCmd: "java -version",
		Issues:       []string{},
		Warnings:     []string{},
		Metadata:     map[string]interface{}{"java_home": "/usr/lib/jvm/java-17"},
		DetectedAt:   time.Now(),
		Duration:     time.Millisecond * 60,
	}, nil
}

func (m *MockRuntimeDetector) DetectAll(ctx context.Context) (*setup.DetectionReport, error) {
	m.DetectAllCalls = append(m.DetectAllCalls, ctx)
	if m.DetectAllFunc != nil {
		return m.DetectAllFunc(ctx)
	}

	goInfo, _ := m.DetectGo(ctx)
	pythonInfo, _ := m.DetectPython(ctx)
	nodejsInfo, _ := m.DetectNodejs(ctx)
	javaInfo, _ := m.DetectJava(ctx)

	return &setup.DetectionReport{
		Timestamp:    time.Now(),
		Platform:     platform.PlatformLinux,
		Architecture: platform.ArchAMD64,
		Runtimes: map[string]*setup.RuntimeInfo{
			"go":     goInfo,
			"python": pythonInfo,
			"nodejs": nodejsInfo,
			"java":   javaInfo,
		},
		Summary: setup.DetectionSummary{
			TotalRuntimes:      4,
			InstalledRuntimes:  4,
			CompatibleRuntimes: 4,
			IssuesFound:        0,
			WarningsFound:      0,
			SuccessRate:        100.0,
			AverageDetectTime:  time.Millisecond * 45,
		},
		Duration:  time.Millisecond * 180,
		Issues:    []string{},
		Warnings:  []string{},
		SessionID: "mock-session-123",
		Metadata:  map[string]interface{}{"mock": true},
	}, nil
}

func (m *MockRuntimeDetector) SetLogger(logger *setup.SetupLogger) {
	m.SetLoggerCalls = append(m.SetLoggerCalls, logger)
	if m.SetLoggerFunc != nil {
		m.SetLoggerFunc(logger)
	}
}

func (m *MockRuntimeDetector) SetTimeout(timeout time.Duration) {
	m.SetTimeoutCalls = append(m.SetTimeoutCalls, timeout)
	if m.SetTimeoutFunc != nil {
		m.SetTimeoutFunc(timeout)
	}
}

func (m *MockRuntimeDetector) Reset() {
	m.DetectGoCalls = make([]context.Context, 0)
	m.DetectPythonCalls = make([]context.Context, 0)
	m.DetectNodejsCalls = make([]context.Context, 0)
	m.DetectJavaCalls = make([]context.Context, 0)
	m.DetectAllCalls = make([]context.Context, 0)
	m.SetLoggerCalls = make([]*setup.SetupLogger, 0)
	m.SetTimeoutCalls = make([]time.Duration, 0)

	m.DetectGoFunc = nil
	m.DetectPythonFunc = nil
	m.DetectNodejsFunc = nil
	m.DetectJavaFunc = nil
	m.DetectAllFunc = nil
	m.SetLoggerFunc = nil
	m.SetTimeoutFunc = nil
}
