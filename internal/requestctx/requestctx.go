package requestctx

import (
	"context"
	"time"
)

type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	requestTimeKey contextKey = "request_time"
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func WithRequestTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, requestTimeKey, t)
}

func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func RequestTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(requestTimeKey).(time.Time); ok {
		return t
	}
	return time.Time{}
}
