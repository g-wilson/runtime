package auth

import (
	"context"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"
)

type bearerKey string

var ctxkey = bearerKey("bearer")

// Bearer represents the authentication state of the request
type Bearer struct {
	UserID    string
	AccountID string
	Scopes    []string
}

// IsNull validates whether the bearer is zero-value (and therefore considered not authenticated)
func (b Bearer) IsNull() bool {
	return b.UserID == ""
}

// MustHaveScope returns an error value if the bearer does not possess a given scope
func (b Bearer) MustHaveScope(scope string) error {
	for _, sc := range b.Scopes {
		if sc == scope {
			return nil
		}
	}

	return hand.New(runtime.ErrCodeAccessDenied)
}

// GetBearerContext returns the identity of the requester
func GetBearerContext(ctx context.Context) Bearer {
	val := ctx.Value(ctxkey)

	if valBearer, ok := val.(Bearer); ok {
		return valBearer
	}

	return Bearer{}
}

// SetBearerContext sets the identity of the requester
func SetBearerContext(ctx context.Context, id Bearer) context.Context {
	return context.WithValue(ctx, ctxkey, id)
}
