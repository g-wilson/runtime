package rpcservice

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

// Service encapsulates an instance of an RPC Service
type Service struct {
	Logger           *logrus.Entry
	Methods          map[string]*Method
	ContextProviders []ContextProvider
	IdentityProvider IdentityContextProvider
}

// NewService creates a Service
func NewService(l *logrus.Entry) *Service {
	return &Service{Logger: l, Methods: make(map[string]*Method)}
}

// WithIdentityProvider attaches an identity provider function to the service
func (s *Service) WithIdentityProvider(idp IdentityContextProvider) *Service {
	s.IdentityProvider = idp
	return s
}

// WithContextProvider attaches a callback function to the request hooks where context can be modified
func (s *Service) WithContextProvider(handler ContextProvider) *Service {
	s.ContextProviders = append(s.ContextProviders, handler)
	return s
}

// AddMethod creates a Method and adds it to the service
func (s *Service) AddMethod(methodName string, handler interface{}, schema gojsonschema.JSONLoader) *Service {
	method := &Method{
		Name:    methodName,
		Handler: handler,
	}

	if schema != nil {
		sc, err := gojsonschema.NewSchema(schema)
		if err != nil {
			panic(fmt.Errorf("runtime cannot parse schema for method %s: %w", methodName, err))
		}

		method.CompiledSchema = sc
	}

	hasReqBody, hasResBody, err := validateMethod(method)
	if err != nil {
		panic(fmt.Errorf("runtime cannot add rpc method %s: %w", methodName, err))
	}

	method.expectsRequestBody = hasReqBody
	method.expectsResponseBody = hasResBody

	s.Methods[methodName] = method
	return s
}

// GetMethod finds an attached Method by name
func (s *Service) GetMethod(methodName string) (*Method, bool) {
	m, ok := s.Methods[methodName]
	return m, ok
}
