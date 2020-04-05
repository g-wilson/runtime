package auth

import (
	"context"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"
)

type identityContextKey string

var ctxkey = identityContextKey("authidentity")

// Claims represents the payload of a JWT access token
type Claims struct {
	Version  string   `json:"v,omitempty"`
	ID       string   `json:"jti,omitempty"`
	Issuer   string   `json:"iss,omitempty"`
	Subject  string   `json:"sub,omitempty"`
	Audience []string `json:"aud,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
}

// MustHaveScope returns an error value if the identity does not possess a given scope
func (tc Claims) MustHaveScope(scope string) error {
	for _, sc := range tc.Scopes {
		if sc == scope {
			return nil
		}
	}

	return hand.New(runtime.ErrCodeAccessDenied)
}

// GetIdentityContext returns the identity of the requester
func GetIdentityContext(ctx context.Context) Claims {
	val := ctx.Value(ctxkey)

	if identity, ok := val.(Claims); ok {
		return identity
	}

	return Claims{}
}

// SetIdentityContext sets the identity of the requester
func SetIdentityContext(ctx context.Context, cl Claims) context.Context {
	return context.WithValue(ctx, ctxkey, cl)
}
