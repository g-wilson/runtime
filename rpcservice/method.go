package rpcservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"

	"github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
)

// Method holds properties about an RPC method as well as the handler function itself
type Method struct {
	Name                string
	Handler             interface{}
	CompiledSchema      *gojsonschema.Schema
	expectsRequestBody  bool
	expectsResponseBody bool
}

// Invoke executes a handler method within a context
func (m *Method) Invoke(ctx context.Context, body []byte) (interface{}, error) {
	startedAt := time.Now()
	reqLogger := logger.FromContext(ctx)

	reqLogger.Update(reqLogger.Entry().WithFields(logrus.Fields{
		"rpc_method": m.Name,
	}))

	handlerValue := reflect.ValueOf(m.Handler)
	handlerType := handlerValue.Type()

	if m.CompiledSchema != nil {
		schemaResult, err := m.CompiledSchema.Validate(gojsonschema.NewBytesLoader(body))
		if err != nil {
			return nil, hand.New("invalid_body")
		}
		if !schemaResult.Valid() {
			errs := schemaResult.Errors()

			var reasons []map[string]string

			for _, err := range errs {
				reasons = append(reasons, map[string]string{
					"field":   err.Field(),
					"type":    err.Type(),
					"message": err.Description(),
				})
			}

			reqLogger.Entry().
				WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
				WithField("err_code", "schema_fail").
				Info("rpc request handled error")

			return nil, hand.New("schema_fail").WithMeta(hand.M{"reasons": reasons})
		}
	}

	var result []reflect.Value

	if len(body) > 0 {
		if !m.expectsRequestBody {
			reqLogger.Entry().
				WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
				WithField("err_code", "request_body_not_expected").
				Info("rpc request handled error")

			return nil, hand.New("invalid_body").WithMessage("rpc method expects no request body")
		}

		req := reflect.New(handlerType.In(1).Elem())
		err := json.Unmarshal(body, req.Interface())
		if err != nil {
			reqLogger.Entry().
				WithError(fmt.Errorf("error parsing request body: %w", err)).
				WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
				Info("request handled error")

			return nil, hand.New("invalid_body").WithMessage("body parsing error")
		}

		result = handlerValue.Call([]reflect.Value{reflect.ValueOf(ctx), req})
	} else {
		if m.expectsRequestBody {
			reqLogger.Entry().
				WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
				WithField("err_code", "request_body_expected").
				Info("rpc request handled error")

			return nil, hand.New("invalid_body").WithMessage("rpc method expects a request body")
		}

		result = handlerValue.Call([]reflect.Value{reflect.ValueOf(ctx)})
	}

	resultErr := result[len(result)-1]

	if resultErr.IsNil() {
		reqLogger.Entry().
			WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
			Info("rpc request handled")

		return result[0].Interface(), nil
	}

	err, _ := resultErr.Interface().(error)

	handErr, ok := err.(hand.E)
	if !ok {
		reqLogger.Entry().
			WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
			WithError(err).
			Error("rpc request unhandled error")

		return nil, hand.New(runtime.ErrCodeUnknown)
	}

	reqLogger.Update(reqLogger.Entry().WithField("err_code", handErr.Code))

	if handErr.Err != nil {
		reqLogger.Update(reqLogger.Entry().WithError(handErr.Err))
	}
	if handErr.Message != "" {
		reqLogger.Update(reqLogger.Entry().WithField("err_message", handErr.Message))
	}

	reqLogger.Entry().
		WithField("handler_duration", time.Now().Sub(startedAt).Microseconds()).
		Warn("rpc request handled error")

	return nil, handErr
}

// validateMethod analyses a Method for requirements before it can be served
// valid handler functions:
// func(ctx context.Context, request *T) (response *T, err error)
// func(ctx context.Context, request *T) (err error)
// func(ctx context.Context) (response *T, err error)
// func(ctx context.Context) (err error)
func validateMethod(method *Method) (hasReqBody, hasResBody bool, err error) {
	handlerValue := reflect.ValueOf(method.Handler)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		err = errors.New("handler must be a function")
		return
	}

	numArgs := handlerType.NumIn()
	numRets := handlerType.NumOut()

	if numArgs < 1 || numArgs > 2 {
		err = fmt.Errorf("handler must have 1 or 2 arguments, %d provided", numArgs)
		return
	}
	if numRets < 1 || numRets > 2 {
		err = fmt.Errorf("handler must have 1 or 2 returns, %d provided", numRets)
		return
	}

	firstArg := handlerType.In(0)
	if !firstArg.Implements(contextType) {
		err = fmt.Errorf("handler first argument must implement context, %s provided", firstArg.Kind())
		return
	}

	lastRet := handlerType.Out(numRets - 1)
	if !lastRet.Implements(errorType) {
		err = fmt.Errorf("handler last return must implement error, %s provided", lastRet.Kind())
		return
	}

	if numArgs == 2 {
		secondArg := handlerType.In(1)

		if secondArg.Kind() != reflect.Ptr {
			err = fmt.Errorf("handler second argument must be pointer, %s provided", secondArg)
			return
		}

		secondArgPtrType := secondArg.Elem()
		if secondArgPtrType.Kind() != reflect.Struct {
			err = fmt.Errorf("handler second argument must be struct, %s", secondArgPtrType.Kind())
			return
		}

		hasReqBody = true

		if method.CompiledSchema == nil {
			err = errors.New("methods with a request type must provide a schema")
			return
		}
	}

	if numRets == 2 {
		firstRet := handlerType.Out(0)

		if firstRet.Kind() != reflect.Ptr {
			err = fmt.Errorf("handler first return must be pointer, %s provided", firstRet)
			return
		}

		firstRetPtrType := firstRet.Elem()
		if firstRetPtrType.Kind() != reflect.Struct {
			err = fmt.Errorf("handler first return must be struct, %s", firstRetPtrType.Kind())
			return
		}

		hasResBody = true
	}

	return
}
