package auth

import (
	"context"
	"strings"
	"time"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

// TokenClaims is used to deserialise
type TokenClaims struct {
	Version   string `json:"v"`
	AccountID string `json:"aid,omitempty"`
	Scope     string `json:"scope"`

	jwt.Claims
}

// Authenticator type is used to validate JWT access tokens and convert them into Bearer
// types which can be used by runtime to evaluate authentication state
type Authenticator struct {
	Keys   *jose.JSONWebKeySet
	Issuer string
}

// NewAuthenticator creates a JWT authenticator
func NewAuthenticator(keys *jose.JSONWebKeySet, issuer string) *Authenticator {
	return &Authenticator{
		Keys:   keys,
		Issuer: issuer,
	}
}

// Authenticate validates the provided JWT access token and returns a Bearer
func (a *Authenticator) Authenticate(ctx context.Context, token string) (*Bearer, error) {
	tok, err := jwt.ParseSigned(token)
	if err != nil {
		return nil, hand.New(runtime.ErrCodeInvalidToken).WithMessage("jwt parse error")
	}

	cl := TokenClaims{}
	if err := tok.Claims(a.Keys, &cl); err != nil {
		return nil, err
	}
	err = cl.Validate(jwt.Expected{
		Issuer: a.Issuer,
		Time:   time.Now().UTC(),
	})
	if err != nil {
		if err == jwt.ErrExpired {
			return nil, hand.New(runtime.ErrCodeInvalidToken).WithMessage("token expired")
		}
		logger.FromContext(ctx).Entry().WithError(err).Warn("jwt validation error")

		return nil, hand.New(runtime.ErrCodeInvalidToken).WithMessage("jwt validation error")
	}

	if cl.Version != "00" {
		return nil, hand.New(runtime.ErrCodeInvalidToken).WithMessage("invalid token version")
	}

	return &Bearer{
		UserID:    cl.Subject,
		AccountID: cl.AccountID,
		Scopes:    strings.Split(cl.Scope, " "),
	}, nil
}
