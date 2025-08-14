package errors

import (
	stderrors "errors"
	"testing"

	internalerrors "lsp-gateway/src/internal/errors"
)

func TestLSPErrorTranslator_MethodNotSupported_FromKeyError(t *testing.T) {
	tr := NewLSPErrorTranslator()
	uerr := tr.CreateUnifiedError("pylsp", "KeyError: x", []string{"workspace", "symbol"})
	if !internalerrors.IsMethodNotSupportedError(uerr) {
		t.Fatalf("expected method-not-supported")
	}
}

func TestLSPErrorTranslator_MethodNotFound_Text(t *testing.T) {
	tr := NewLSPErrorTranslator()
	uerr := tr.CreateUnifiedError("go", "Method not found: textDocument/references", nil)
	if !internalerrors.IsMethodNotSupportedError(uerr) {
		t.Fatalf("expected method-not-supported")
	}
}

func TestLSPErrorTranslator_TranslateToUnified(t *testing.T) {
	tr := NewLSPErrorTranslator()
	e := tr.TranslateToUnifiedError("go", internalerrors.NewTimeoutError("op", "go", 0, nil))
	if !internalerrors.IsTimeoutError(e) {
		t.Fatalf("expected timeout")
	}
	e2 := tr.TranslateToUnifiedError("go", stderrors.New("connection refused"))
	if !internalerrors.IsConnectionError(e2) {
		t.Fatalf("expected connection")
	}
}
