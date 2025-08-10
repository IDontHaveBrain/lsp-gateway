package common

import (
    "context"
    "time"

    "lsp-gateway/src/internal/constants"
)

func CreateContext(duration time.Duration) (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), duration)
}

// WithTimeout derives a timeout from a parent context (nil -> Background)
func WithTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
    if parent == nil {
        parent = context.Background()
    }
    return context.WithTimeout(parent, duration)
}

func CreateContextWithDefault() (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
}
