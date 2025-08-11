package base

import (
	"context"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
)

type OperationType string

const (
	OperationInitialize OperationType = "initialize"
	OperationRequest    OperationType = "request"
	OperationShutdown   OperationType = "shutdown"
)

type TimeoutManager struct {
	operationType    OperationType
	customTimeouts   map[string]time.Duration
	globalMultiplier float64
}

func NewTimeoutManager() *TimeoutManager {
	return &TimeoutManager{
		operationType:    OperationRequest,
		customTimeouts:   make(map[string]time.Duration),
		globalMultiplier: 1.0,
	}
}

func (tm *TimeoutManager) ForOperation(opType OperationType) *TimeoutManager {
	tm.operationType = opType
	return tm
}

func (tm *TimeoutManager) WithCustomTimeout(language string, timeout time.Duration) *TimeoutManager {
	tm.customTimeouts[language] = timeout
	return tm
}

func (tm *TimeoutManager) WithGlobalMultiplier(multiplier float64) *TimeoutManager {
	tm.globalMultiplier = multiplier
	return tm
}

func (tm *TimeoutManager) GetTimeout(language string) time.Duration {
	if customTimeout, exists := tm.customTimeouts[language]; exists {
		return time.Duration(float64(customTimeout) * tm.globalMultiplier)
	}

	var timeout time.Duration

	switch tm.operationType {
	case OperationInitialize:
		timeout = constants.GetInitializeTimeout(language)
	case OperationRequest:
		timeout = constants.GetRequestTimeout(language)
	case OperationShutdown:
		timeout = constants.ProcessShutdownTimeout
	default:
		common.LSPLogger.Warn("[TimeoutManager] Unknown operation type %s, using request timeout", string(tm.operationType))
		timeout = constants.GetRequestTimeout(language)
	}

	if timeout == 0 {
		common.LSPLogger.Warn("[TimeoutManager] Zero timeout calculated for language %s, using default", language)
		timeout = constants.DefaultRequestTimeout
	}

	return time.Duration(float64(timeout) * tm.globalMultiplier)
}

func (tm *TimeoutManager) CreateContext(ctx context.Context, language string) (context.Context, context.CancelFunc) {
	timeout := tm.GetTimeout(language)
	return common.WithTimeout(ctx, timeout)
}

func (tm *TimeoutManager) GetOverallTimeout(languages []string) time.Duration {
	if len(languages) == 0 {
		return tm.GetTimeout("")
	}

	maxTimeout := time.Duration(0)
	for _, lang := range languages {
		langTimeout := tm.GetTimeout(lang)
		if langTimeout > maxTimeout {
			maxTimeout = langTimeout
		}
	}

	if maxTimeout == 0 {
		switch tm.operationType {
		case OperationInitialize:
			maxTimeout = constants.DefaultInitializeTimeout
		case OperationShutdown:
			maxTimeout = constants.ProcessShutdownTimeout
		default:
			maxTimeout = constants.DefaultRequestTimeout
		}
		common.LSPLogger.Warn("[TimeoutManager] No valid timeouts found for languages %v, using default %v", languages, maxTimeout)
	}

	return maxTimeout
}

func (tm *TimeoutManager) CreateOverallContext(ctx context.Context, languages []string) (context.Context, context.CancelFunc) {
	timeout := tm.GetOverallTimeout(languages)
	return common.WithTimeout(ctx, timeout)
}

func (tm *TimeoutManager) LogTimeoutInfo(language string) {
	timeout := tm.GetTimeout(language)
	common.LSPLogger.Debug("[TimeoutManager] %s timeout for %s: %v", string(tm.operationType), language, timeout)
}

func (tm *TimeoutManager) GetSupportedLanguages() []string {
	return []string{"java", "python", "go", "javascript", "typescript", "rust"}
}
