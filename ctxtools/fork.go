package ctxtools

import (
	"context"
	"time"

	"github.com/g-wilson/runtime/logger"
)

// ForkWithTimeout creates a new context with a specified timeout but clones the logger and request ID into the new context
func ForkWithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) {
	requestID := GetRequestID(ctx)
	logInstance := logger.FromContext(ctx)

	newCtx := context.Background()

	newCtx = logger.SetContext(newCtx, logInstance.Entry())
	newCtx = SetRequestID(newCtx, requestID)

	newCtx, cancel := context.WithTimeout(newCtx, timeout)

	go func() {
		defer cancel()
		err := fn(newCtx)
		if err != nil {
			logger.FromContext(newCtx).Entry().WithError(err).Error("forked context errored")
		}
	}()
}
