package devserver

import (
	"context"
	"time"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

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

// Authenticate validates the provided JWT access token and returns the claims
func (a *Authenticator) Authenticate(ctx context.Context, token string) (map[string]interface{}, error) {
	tok, err := jwt.ParseSigned(token)
	if err != nil {
		return nil, hand.New(runtime.ErrCodeInvalidToken).WithMessage("jwt parse error")
	}

	cl := jwt.Claims{}
	if err := tok.Claims(a.Keys, &cl); err != nil {
		return nil, err
	}
	err = cl.Validate(jwt.Expected{
		Issuer: a.Issuer,
		Time:   time.Now().UTC(),
	})
	if err != nil {
		var msg string

		switch true {
		case err == jwt.ErrInvalidClaims:
			msg = "invalid claims"
		case err == jwt.ErrInvalidIssuer:
			msg = "invalid issuer"
		case err == jwt.ErrInvalidSubject:
			msg = "invalid subject"
		case err == jwt.ErrInvalidAudience:
			msg = "invalid audience"
		case err == jwt.ErrInvalidID:
			msg = "invalid id"
		case err == jwt.ErrNotValidYet:
			msg = "not valid yet"
		case err == jwt.ErrExpired:
			msg = "expired"
		case err == jwt.ErrIssuedInTheFuture:
			msg = "issued in future"
		case err == jwt.ErrInvalidContentType:
			msg = "invalid content type"
		default:
			msg = "jwt validation error"
		}

		return nil, hand.New(runtime.ErrCodeInvalidToken).WithMessage(msg)
	}

	svcClaims := map[string]interface{}{}
	tok.UnsafeClaimsWithoutVerification(&svcClaims)

	return svcClaims, nil
}
