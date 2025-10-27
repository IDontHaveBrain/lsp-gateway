package search

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSearchGuardWithEnabledGuard(t *testing.T) {
	guard := NewSearchGuard(true)

	value, err := guard.WithEnabledGuard(func() (interface{}, error) {
		return "ok", nil
	})

	require.NoError(t, err)
	require.Equal(t, "ok", value)
}

func TestSearchGuardWithEnabledGuardDisabled(t *testing.T) {
	guard := NewSearchGuard(false)
	called := false

	value, err := guard.WithEnabledGuard(func() (interface{}, error) {
		called = true
		return "ok", nil
	})

	require.NoError(t, err)
	require.False(t, called)
	require.Nil(t, value)
}

func TestSearchGuardWithSearchResponseDisabled(t *testing.T) {
	guard := NewSearchGuard(false)

	response, err := guard.WithSearchResponse(SearchTypeSymbol, func() (*SearchResponse, error) {
		return &SearchResponse{Type: string(SearchTypeSymbol)}, nil
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Empty(t, response.Results)
	require.NotNil(t, response.Metadata)
	require.False(t, response.Metadata.CacheEnabled)
}

func TestSearchGuardWithEnhancedSymbolResultDisabled(t *testing.T) {
	guard := NewSearchGuard(false)
	query := &EnhancedSymbolQuery{Pattern: "test"}

	response, err := guard.WithEnhancedSymbolResult(query, func() (*EnhancedSymbolSearchResponse, error) {
		return &EnhancedSymbolSearchResponse{}, nil
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, query, response.Query)
	require.NotNil(t, response.Metadata)
	require.False(t, response.Metadata.CacheEnabled)
}

func TestSearchGuardMustBeEnabled(t *testing.T) {
	guard := NewSearchGuard(false)
	require.Error(t, guard.MustBeEnabled())

	guard = NewSearchGuard(true)
	require.NoError(t, guard.MustBeEnabled())
}

func TestSearchGuardWithErrorResult(t *testing.T) {
	guard := NewSearchGuard(true)
	expectedErr := errors.New("fail")

	_, err := guard.WithErrorResult(func() ([]interface{}, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSearchGuardWithSymbolInfoResultDisabled(t *testing.T) {
	guard := NewSearchGuard(false)

	result, err := guard.WithSymbolInfoResult("symbol", func() (*SymbolInfoResponse, error) {
		return &SymbolInfoResponse{}, nil
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "symbol", result.SymbolName)
	require.NotNil(t, result.Metadata)
	require.False(t, result.Metadata.CacheEnabled)
}

func TestSearchGuardWithReferenceResultDisabled(t *testing.T) {
	guard := NewSearchGuard(false)
	options := &ReferenceSearchOptions{}

	result, err := guard.WithReferenceResult("symbol", options, func() (*ReferenceSearchResponse, error) {
		return &ReferenceSearchResponse{}, nil
	})

	require.NoError(t, err)
	require.Equal(t, "symbol", result.SymbolName)
	require.NotNil(t, result.Metadata)
	require.False(t, result.Metadata.CacheEnabled)
}

func TestSearchGuardWithEnabledGuardError(t *testing.T) {
	guard := NewSearchGuard(true)
	expectedErr := errors.New("fail")

	_, err := guard.WithEnabledGuard(func() (interface{}, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSearchGuardWithSearchResponseError(t *testing.T) {
	guard := NewSearchGuard(true)
	expectedErr := errors.New("fail")

	_, err := guard.WithSearchResponse(SearchTypeSymbol, func() (*SearchResponse, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSearchGuardWithEnhancedSymbolResultError(t *testing.T) {
	guard := NewSearchGuard(true)
	expectedErr := errors.New("fail")

	_, err := guard.WithEnhancedSymbolResult(&EnhancedSymbolQuery{}, func() (*EnhancedSymbolSearchResponse, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSearchGuardWithSymbolInfoResultError(t *testing.T) {
	guard := NewSearchGuard(true)
	expectedErr := errors.New("fail")

	_, err := guard.WithSymbolInfoResult("symbol", func() (*SymbolInfoResponse, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSearchGuardWithReferenceResultError(t *testing.T) {
	guard := NewSearchGuard(true)
	expectedErr := errors.New("fail")

	_, err := guard.WithReferenceResult("symbol", &ReferenceSearchOptions{}, func() (*ReferenceSearchResponse, error) {
		return nil, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestSearchGuardWithEnabledGuardNilFn(t *testing.T) {
	guard := NewSearchGuard(true)

	_, err := guard.WithEnabledGuard(nil)
	require.Error(t, err)
}

func TestSearchGuardWithEnabledGuardTiming(t *testing.T) {
	guard := NewSearchGuard(true)

	start := time.Now()
	value, err := guard.WithEnabledGuard(func() (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "done", nil
	})

	require.NoError(t, err)
	require.Equal(t, "done", value)
	require.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond)
}
