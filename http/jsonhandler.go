package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/g-wilson/runtime/hand"

	"github.com/aws/aws-lambda-go/events"
	"github.com/xeipuuv/gojsonschema"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

type JSONHandler struct {
	innerFunc           interface{}
	expectsRequestBody  bool
	expectsResponseBody bool
	compiledSchema      *gojsonschema.Schema
}

func NewJSONHandler(innerFunc interface{}, schema gojsonschema.JSONLoader) (JSONHandler, error) {
	h := JSONHandler{
		innerFunc: innerFunc,
	}

	if schema != nil {
		sc, err := gojsonschema.NewSchema(schema)
		if err != nil {
			return JSONHandler{}, fmt.Errorf("JSONHandler: cannot parse schema for endpoint: %w", err)
		}

		h.compiledSchema = sc
	}

	hasReqBody, hasResBody, err := validateJSONHandler(h)
	if err != nil {
		return JSONHandler{}, fmt.Errorf("JSONHandler: cannot create endpoint: %w", err)
	}

	h.expectsRequestBody = hasReqBody
	h.expectsResponseBody = hasResBody

	return h, nil
}

func (h JSONHandler) Handle(ctx context.Context, event events.APIGatewayV2HTTPRequest) (res events.APIGatewayProxyResponse, err error) {
	result, err := h.invoke(ctx, []byte(event.Body))
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	if result == nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNoContent,
			Body:       "",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(result),
		Headers: map[string]string{
			"Content-Type": "application/json; charset=utf-8",
		},
	}, nil
}

func (h JSONHandler) invoke(ctx context.Context, requestBodyBytes []byte) ([]byte, error) {
	handlerValue := reflect.ValueOf(h.innerFunc)
	handlerType := handlerValue.Type()

	if h.compiledSchema != nil {
		schemaResult, err := h.compiledSchema.Validate(gojsonschema.NewBytesLoader(requestBodyBytes))
		if err != nil {
			return nil, hand.New(hand.ErrCodeInvalidBody)
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

			err := hand.New(hand.ErrCodeSchemaFailure).WithMeta(hand.M{"reasons": reasons})

			return nil, err
		}
	}

	var result []reflect.Value

	if len(requestBodyBytes) > 0 {
		if !h.expectsRequestBody {
			return nil, hand.New(hand.ErrCodeInvalidBody).WithMessage("unexpected request body")
		}

		req := reflect.New(handlerType.In(1).Elem())
		err := json.Unmarshal(requestBodyBytes, req.Interface())
		if err != nil {
			return nil, hand.Wrap(hand.ErrCodeInvalidBody, fmt.Errorf("error parsing request body: %w", err)).
				WithMessage("unable to parse body")
		}

		result = handlerValue.Call([]reflect.Value{reflect.ValueOf(ctx), req})
	} else {
		if h.expectsRequestBody {
			return nil, hand.New(hand.ErrCodeInvalidBody).WithMessage("expecting request body")
		}

		result = handlerValue.Call([]reflect.Value{reflect.ValueOf(ctx)})
	}

	resultErr := result[len(result)-1]

	if resultErr.IsNil() {
		if len(result) == 1 {
			return nil, nil
		}

		resBytes, err := json.Marshal(result[0].Interface())
		if err == nil {
			return resBytes, nil
		}

		return nil, fmt.Errorf("JSONHandler: could not serialise response body: %w", err)
	}

	return nil, resultErr.Interface().(error)
}

// validateJSONHandler analyses a JSONHandler for requirements before it can be served
// valid handler functions:
// func(ctx context.Context, request *T) (response *T, err error)
// func(ctx context.Context, request *T) (err error)
// func(ctx context.Context) (response *T, err error)
// func(ctx context.Context) (err error)
func validateJSONHandler(h JSONHandler) (hasReqBody, hasResBody bool, err error) {
	handlerValue := reflect.ValueOf(h.innerFunc)
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
			err = fmt.Errorf("handler second argument must be struct, %s provided", secondArgPtrType.Kind())
			return
		}

		hasReqBody = true

		if h.compiledSchema == nil {
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
