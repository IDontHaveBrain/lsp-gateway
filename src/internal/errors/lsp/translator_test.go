package lsp

import (
	stderrors "errors"
	"testing"

	"lsp-gateway/src/internal/errors"
)

func TestLSPErrorTranslator_MethodNotSupported_FromKeyError(t *testing.T) {
	uerr := CreateUnifiedError("pylsp", "KeyError: x", []string{"workspace", "symbol"})
	if !errors.IsMethodNotSupportedError(uerr) {
		t.Fatalf("expected method-not-supported")
	}
}

func TestLSPErrorTranslator_MethodNotFound_Text(t *testing.T) {
	uerr := CreateUnifiedError("go", "Method not found: textDocument/references", nil)
	if !errors.IsMethodNotSupportedError(uerr) {
		t.Fatalf("expected method-not-supported")
	}
}

func TestLSPErrorTranslator_TranslateToUnified(t *testing.T) {
	e := TranslateToUnifiedError("go", errors.NewTimeoutError("op", "go", 0, nil))
	if !errors.IsTimeoutError(e) {
		t.Fatalf("expected timeout")
	}
	e2 := TranslateToUnifiedError("go", stderrors.New("connection refused"))
	if !errors.IsConnectionError(e2) {
		t.Fatalf("expected connection")
	}
}
