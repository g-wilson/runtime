package rpcmethod

import (
	"context"
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// ContextProvider is a function which is called before the request
type ContextProvider func(ctx context.Context) context.Context

// IdentityContextProvider is a special context provider which has an argument for the access token claims of the current request
type IdentityContextProvider func(ctx context.Context, claims map[string]interface{}) context.Context

// Method holds properties about an RPC method as well as the handler function itself
type Method struct {
	Name string

	logger              *logrus.Entry
	handler             interface{}
	expectsRequestBody  bool
	expectsResponseBody bool
	compiledSchema      *gojsonschema.Schema
	contextProviders    []ContextProvider
	identityProvider    IdentityContextProvider
}

type Params struct {
	Logger  *logrus.Entry
	Name    string
	Handler interface{}
	Schema  gojsonschema.JSONLoader
}

// New creates a Method from some params
func New(params Params) *Method {
	method := &Method{
		Name:    params.Name,
		logger:  params.Logger,
		handler: params.Handler,
	}

	if params.Schema != nil {
		sc, err := gojsonschema.NewSchema(params.Schema)
		if err != nil {
			panic(fmt.Errorf("runtime cannot parse schema for method %s: %w", method.Name, err))
		}

		method.compiledSchema = sc
	}

	hasReqBody, hasResBody, err := validate(method)
	if err != nil {
		panic(fmt.Errorf("runtime cannot create method %s: %w", method.Name, err))
	}

	method.expectsRequestBody = hasReqBody
	method.expectsResponseBody = hasResBody

	return method
}

// WithIdentityProvider attaches an identity provider function to the Method
func (m *Method) WithIdentityProvider(idp IdentityContextProvider) *Method {
	m.identityProvider = idp
	return m
}

// WithContextProvider attaches a callback function to the request hooks where context can be modified
func (m *Method) WithContextProvider(handler ContextProvider) *Method {
	m.contextProviders = append(m.contextProviders, handler)
	return m
}
