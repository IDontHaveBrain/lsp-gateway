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
