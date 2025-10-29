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

func TestWithEnabledGuardNilFn(t *testing.T) {
	_, err := WithEnabledGuard[int](true, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil function")
}

func TestWithEnabledGuardDefaultEnabled(t *testing.T) {
	callCount := 0

	result, err := WithEnabledGuardDefault(true, func() (string, error) {
		callCount++
		return "success", nil
	}, "default")

	require.NoError(t, err)
	require.Equal(t, 1, callCount)
	require.Equal(t, "success", result)
}

func TestWithEnabledGuardDefaultDisabled(t *testing.T) {
	callCount := 0

	result, err := WithEnabledGuardDefault(false, func() (string, error) {
		callCount++
		return "success", nil
	}, "default")

	require.NoError(t, err)
	require.Equal(t, 0, callCount)
	require.Equal(t, "default", result)
}

func TestWithEnabledGuardDefaultError(t *testing.T) {
	expectedErr := errors.New("fail")

	_, err := WithEnabledGuardDefault(true, func() (string, error) {
		return "", expectedErr
	}, "default")

	require.ErrorIs(t, err, expectedErr)
}

func TestWithEnabledGuardDefaultNilFn(t *testing.T) {
	result, err := WithEnabledGuardDefault(true, nil, "default")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil function")
	require.Equal(t, "default", result)
}

func TestWithEnabledGuardDefaultComplexType(t *testing.T) {
	type Response struct {
		Value string
		Count int
	}

	defaultResp := Response{Value: "default", Count: 0}

	result, err := WithEnabledGuardDefault(false, func() (Response, error) {
		return Response{Value: "actual", Count: 10}, nil
	}, defaultResp)

	require.NoError(t, err)
	require.Equal(t, defaultResp, result)
}

func TestWithEnabledGuard3Enabled(t *testing.T) {
	callCount := 0

	value, found, err := WithEnabledGuard3(true, func() (string, bool, error) {
		callCount++
		return "result", true, nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, callCount)
	require.Equal(t, "result", value)
	require.True(t, found)
}

func TestWithEnabledGuard3Disabled(t *testing.T) {
	callCount := 0

	value, found, err := WithEnabledGuard3(false, func() (string, bool, error) {
		callCount++
		return "result", true, nil
	})

	require.NoError(t, err)
	require.Equal(t, 0, callCount)
	require.Equal(t, "", value)
	require.False(t, found)
}

func TestWithEnabledGuard3Error(t *testing.T) {
	expectedErr := errors.New("fail")

	_, _, err := WithEnabledGuard3(true, func() (int, bool, error) {
		return 0, false, expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
}

func TestWithEnabledGuard3NilFn(t *testing.T) {
	_, found, err := WithEnabledGuard3[int](true, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil function")
	require.False(t, found)
}

func TestWithEnabledGuardOrErrorEnabled(t *testing.T) {
	callCount := 0

	result, err := WithEnabledGuardOrError(true, func() (int, error) {
		callCount++
		return 42, nil
	}, "custom error")

	require.NoError(t, err)
	require.Equal(t, 1, callCount)
	require.Equal(t, 42, result)
}

func TestWithEnabledGuardOrErrorDisabled(t *testing.T) {
	callCount := 0

	result, err := WithEnabledGuardOrError(false, func() (int, error) {
		callCount++
		return 42, nil
	}, "custom error")

	require.Error(t, err)
	require.Contains(t, err.Error(), "custom error")
	require.Equal(t, 0, callCount)
	require.Equal(t, 0, result)
}

func TestWithEnabledGuardOrErrorDisabledDefaultMsg(t *testing.T) {
	_, err := WithEnabledGuardOrError(false, func() (int, error) {
		return 42, nil
	}, "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "operation disabled")
}

func TestWithEnabledGuardOrErrorPropagation(t *testing.T) {
	expectedErr := errors.New("fail")

	_, err := WithEnabledGuardOrError(true, func() (string, error) {
		return "", expectedErr
	}, "custom error")

	require.ErrorIs(t, err, expectedErr)
}

func TestWithEnabledGuardOrErrorNilFn(t *testing.T) {
	_, err := WithEnabledGuardOrError[int](true, nil, "custom error")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil function")
}
