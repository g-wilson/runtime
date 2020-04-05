package rpcservice

import (
	"encoding/json"
	"testing"

	"github.com/g-wilson/runtime/auth"

	"github.com/stretchr/testify/assert"
)

func TestCreateAuthIdentity(t *testing.T) {
	t.Run("no scope", func(t *testing.T) {
		authDataJSONStr := []byte(`{
            "claims": {
                "aud": "[client_222 client_111]",
                "exp": "1.576422538e+09",
                "iat": "1.576418938e+09",
                "iss": "https://identity.example.com",
                "nbf": "1.576418938e+09",
                "sub": "user_123",
                "v": "00"
            },
            "scopes": null
		}`)

		var authData = map[string]interface{}{}
		json.Unmarshal(authDataJSONStr, &authData)

		b, _ := createAuthIdentity(authData)

		assert.Equal(t, auth.Claims{
			Version:  "00",
			Issuer:   "https://identity.example.com",
			Subject:  "user_123",
			Audience: []string{"client_222", "client_111"},
		}, b)
	})

	t.Run("yes scope", func(t *testing.T) {
		authDataJSONStr := []byte(`{
            "claims": {
                "aud": "[client_222 client_111]",
                "exp": "1.576422538e+09",
                "iat": "1.576418938e+09",
                "iss": "https://identity.example.com",
                "nbf": "1.576418938e+09",
                "sub": "user_123",
                "v": "00"
            },
            "scopes": [
				"one",
				"two"
			]
		}`)

		var authData = map[string]interface{}{}
		json.Unmarshal(authDataJSONStr, &authData)

		b, _ := createAuthIdentity(authData)

		assert.Equal(t, auth.Claims{
			Version:  "00",
			Issuer:   "https://identity.example.com",
			Subject:  "user_123",
			Audience: []string{"client_222", "client_111"},
			Scopes:   []string{"one", "two"},
		}, b)
	})

	t.Run("internal client", func(t *testing.T) {
		authDataJSONStr := []byte(`{
            "claims": {
                "aud": "[client_222]",
                "exp": "1.576422538e+09",
                "iat": "1.576418938e+09",
                "iss": "https://identity.example.com",
                "nbf": "1.576418938e+09",
                "sub": "some_other_service",
                "v": "00"
            },
            "scopes": [
				"system"
			]
		}`)

		var authData = map[string]interface{}{}
		json.Unmarshal(authDataJSONStr, &authData)

		b, _ := createAuthIdentity(authData)

		assert.Equal(t, auth.Claims{
			Version:  "00",
			Issuer:   "https://identity.example.com",
			Subject:  "some_other_service",
			Audience: []string{"client_222"},
			Scopes:   []string{"system"},
		}, b)
	})
}
