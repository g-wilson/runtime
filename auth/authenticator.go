package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

// OpenIDConfig provides the fields we need from an openid configuration file
type OpenIDConfig struct {
	Issuer  string `json:"issuer"`
	JwksURI string `json:"jwks_uri"`
}

// Authenticator type is used to validate JWT access tokens and convert them into Bearer
// types which can be used by runtime to evaluate authentication state
type Authenticator struct {
	Keys   *jose.JSONWebKeySet
	Issuer string
}

// Authenticate validates the provided JWT access token and scans the claims
func (a *Authenticator) Authenticate(ctx context.Context, token string, dest interface{}) error {
	tok, err := jwt.ParseSigned(token)
	if err != nil {
		return hand.New(runtime.ErrCodeInvalidToken).WithMessage("jwt parse error")
	}

	cl := jwt.Claims{}
	if err := tok.Claims(a.Keys, &cl); err != nil {
		return err
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

		return hand.New(runtime.ErrCodeInvalidToken).WithMessage(msg)
	}

	return tok.UnsafeClaimsWithoutVerification(dest)
}

// New creates a JWT authenticator from an OpenID configuration URL
func New(configURL string) (a *Authenticator, err error) {
	var keyset jose.JSONWebKeySet
	var config OpenIDConfig

	err = getJSON(configURL, &config)
	if err != nil {
		return
	}

	err = getJSON(config.JwksURI, &keyset)
	if err != nil {
		return
	}

	a = &Authenticator{
		Keys:   &keyset,
		Issuer: config.Issuer,
	}

	return
}

func getJSON(url string, dest interface{}) (err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	resBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("http 200 expected")
	}
	if len(resBytes) == 0 {
		return errors.New("response body expected")
	}

	err = json.Unmarshal(resBytes, dest)

	return
}
