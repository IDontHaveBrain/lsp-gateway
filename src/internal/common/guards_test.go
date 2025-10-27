package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithEnabledGuardSuccess(t *testing.T) {
	callCount := 0

	result, err := WithEnabledGuard(true, func() (int, error) {
		callCount++
		return 42, nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, callCount)
	require.Equal(t, 42, result)
}

func TestWithEnabledGuardDisabled(t *testing.T) {
	callCount := 0

	result, err := WithEnabledGuard(false, func() (int, error) {
		callCount++
		return 10, nil
	})

	require.NoError(t, err)
	require.Equal(t, 0, callCount)
	require.Equal(t, 0, result)
}

func TestWithEnabledGuardErrorPropagation(t *testing.T) {
	expectedErr := errors.New("boom")

	_, err := WithEnabledGuard(true, func() (int, error) {
		return 0, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}
