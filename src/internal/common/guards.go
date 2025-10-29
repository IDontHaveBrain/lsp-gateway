package common

import "errors"

func WithEnabledGuard[T any](enabled bool, fn func() (T, error)) (T, error) {
	var zero T
	if !enabled {
		return zero, nil
	}
	if fn == nil {
		return zero, errors.New("nil function")
	}
	return fn()
}

func WithEnabledGuardDefault[T any](enabled bool, fn func() (T, error), defaultValue T) (T, error) {
	if !enabled {
		return defaultValue, nil
	}
	if fn == nil {
		return defaultValue, errors.New("nil function")
	}
	return fn()
}

func WithEnabledGuard3[T any](enabled bool, fn func() (T, bool, error)) (T, bool, error) {
	var zero T
	if !enabled {
		return zero, false, nil
	}
	if fn == nil {
		return zero, false, errors.New("nil function")
	}
	return fn()
}

func WithEnabledGuardOrError[T any](enabled bool, fn func() (T, error), errMsg string) (T, error) {
	var zero T
	if !enabled {
		if errMsg == "" {
			errMsg = "operation disabled"
		}
		return zero, errors.New(errMsg)
	}
	if fn == nil {
		return zero, errors.New("nil function")
	}
	return fn()
}
