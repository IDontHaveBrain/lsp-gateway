package errors

import (
	"errors"
	"testing"
)

func TestClassifyByPattern(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantClass ErrorClassification
	}{
		{
			name:      "nil error",
			err:       nil,
			wantClass: ClassUnknown,
		},
		{
			name:      "connection error - connection",
			err:       errors.New("connection failed"),
			wantClass: ClassConnection,
		},
		{
			name:      "connection error - refused",
			err:       errors.New("connection refused"),
			wantClass: ClassConnection,
		},
		{
			name:      "connection error - broken pipe",
			err:       errors.New("broken pipe"),
			wantClass: ClassConnection,
		},
		{
			name:      "validation error - parameter",
			err:       errors.New("invalid parameter"),
			wantClass: ClassValidation,
		},
		{
			name:      "validation error - invalid params",
			err:       errors.New("invalid params"),
			wantClass: ClassValidation,
		},
		{
			name:      "timeout error - timeout",
			err:       errors.New("operation timeout"),
			wantClass: ClassTimeout,
		},
		{
			name:      "timeout error - deadline exceeded",
			err:       errors.New("deadline exceeded"),
			wantClass: ClassTimeout,
		},
		{
			name:      "cancellation error - canceled",
			err:       errors.New("request canceled"),
			wantClass: ClassCancellation,
		},
		{
			name:      "cancellation error - cancelled (UK spelling)",
			err:       errors.New("request cancelled"),
			wantClass: ClassCancellation,
		},
		{
			name:      "method not supported - not supported",
			err:       errors.New("method not supported"),
			wantClass: ClassMethodNotSupported,
		},
		{
			name:      "method not supported - method not found",
			err:       errors.New("method not found"),
			wantClass: ClassMethodNotSupported,
		},
		{
			name:      "process error - process",
			err:       errors.New("process failed"),
			wantClass: ClassProcess,
		},
		{
			name:      "process error - no such file",
			err:       errors.New("no such file"),
			wantClass: ClassProcess,
		},
		{
			name:      "protocol error - json",
			err:       errors.New("json parse error"),
			wantClass: ClassProtocol,
		},
		{
			name:      "protocol error - unmarshal",
			err:       errors.New("failed to unmarshal"),
			wantClass: ClassProtocol,
		},
		{
			name:      "unknown error",
			err:       errors.New("some random error"),
			wantClass: ClassUnknown,
		},
		{
			name:      "case insensitive - Connection",
			err:       errors.New("Connection failed"),
			wantClass: ClassConnection,
		},
		{
			name:      "case insensitive - TIMEOUT",
			err:       errors.New("TIMEOUT occurred"),
			wantClass: ClassTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyByPattern(tt.err)
			if got != tt.wantClass {
				t.Errorf("ClassifyByPattern() = %v, want %v", got, tt.wantClass)
			}
		})
	}
}

func TestGetPatterns(t *testing.T) {
	tests := []struct {
		name           string
		classification ErrorClassification
		wantNonEmpty   bool
	}{
		{
			name:           "connection patterns",
			classification: ClassConnection,
			wantNonEmpty:   true,
		},
		{
			name:           "validation patterns",
			classification: ClassValidation,
			wantNonEmpty:   true,
		},
		{
			name:           "timeout patterns",
			classification: ClassTimeout,
			wantNonEmpty:   true,
		},
		{
			name:           "unknown classification",
			classification: ClassUnknown,
			wantNonEmpty:   false,
		},
		{
			name:           "invalid classification",
			classification: ErrorClassification("nonexistent"),
			wantNonEmpty:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPatterns(tt.classification)
			if tt.wantNonEmpty && len(got) == 0 {
				t.Errorf("GetPatterns() returned empty slice, want non-empty")
			}
			if !tt.wantNonEmpty && got != nil {
				t.Errorf("GetPatterns() = %v, want nil", got)
			}
		})
	}
}

func TestGetPatternsReturnsACopy(t *testing.T) {
	patterns1 := GetPatterns(ClassConnection)
	patterns2 := GetPatterns(ClassConnection)

	if len(patterns1) == 0 {
		t.Fatal("Expected non-empty patterns")
	}

	patterns1[0] = "modified"

	if patterns1[0] == patterns2[0] {
		t.Error("GetPatterns() should return a copy, not the original slice")
	}
}
