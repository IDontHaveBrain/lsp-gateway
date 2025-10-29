package errors

import (
	"strings"
)

type ErrorClassification string

const (
	ClassConnection         ErrorClassification = "connection"
	ClassValidation         ErrorClassification = "validation"
	ClassTimeout            ErrorClassification = "timeout"
	ClassCancellation       ErrorClassification = "cancellation"
	ClassMethodNotSupported ErrorClassification = "method_not_supported"
	ClassProcess            ErrorClassification = "process"
	ClassProtocol           ErrorClassification = "protocol"
	ClassUnknown            ErrorClassification = "unknown"
)

var errorPatterns = map[ErrorClassification][]string{
	ClassConnection: {
		"connection", "connect", "network", "dial",
		"refused", "unreachable", "broken pipe",
	},
	ClassValidation: {
		"validation", "parameter", "invalid params",
	},
	ClassTimeout: {
		"timeout", "deadline exceeded", "context deadline exceeded",
	},
	ClassCancellation: {
		"canceled", "cancelled", "context canceled",
	},
	ClassMethodNotSupported: {
		"not supported", "unsupported", "method not found",
		"methodnotfound", "not implemented", "capability not available",
	},
	ClassProcess: {
		"process", "executable", "no such file",
	},
	ClassProtocol: {
		"json", "rpc", "protocol", "invalid response",
		"parse", "unmarshal", "decode",
	},
}

func ClassifyByPattern(err error) ErrorClassification {
	if err == nil {
		return ClassUnknown
	}

	errMsg := strings.ToLower(err.Error())

	for classification, patterns := range errorPatterns {
		for _, pattern := range patterns {
			if strings.Contains(errMsg, pattern) {
				return classification
			}
		}
	}

	return ClassUnknown
}

func GetPatterns(classification ErrorClassification) []string {
	if patterns, ok := errorPatterns[classification]; ok {
		result := make([]string, len(patterns))
		copy(result, patterns)
		return result
	}
	return nil
}
