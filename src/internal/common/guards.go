package common

func WithEnabledGuard[T any](enabled bool, fn func() (T, error)) (T, error) {
    var zero T
    if !enabled {
        return zero, nil
    }
    return fn()
}

