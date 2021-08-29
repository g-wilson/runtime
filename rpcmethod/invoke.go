package rpcmethod

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"

	"github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
)

// Invoke executes the method
func (m *Method) Invoke(ctx context.Context, body []byte) (interface{}, error) {
	startedAt := time.Now()
	reqLogger := logger.FromContext(ctx)

	reqLogger.Update(reqLogger.Entry().WithFields(logrus.Fields{
		"rpc_method": m.Name,
	}))

	handlerValue := reflect.ValueOf(m.handler)
	handlerType := handlerValue.Type()

	if m.compiledSchema != nil {
		schemaResult, err := m.compiledSchema.Validate(gojsonschema.NewBytesLoader(body))
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

			err := hand.New("schema_fail").WithMeta(hand.M{"reasons": reasons})

			reqLogger.Entry().
				WithError(err).
				WithField("handler_duration", getDuration(startedAt)).
				Warn("rpc request handled error")

			return nil, err
		}
	}

	var result []reflect.Value

	if len(body) > 0 {
		if !m.expectsRequestBody {
			reqLogger.Entry().
				WithError(hand.New("request_body_not_expected")).
				WithField("handler_duration", getDuration(startedAt)).
				Warn("rpc request handled error")

			return nil, hand.New("invalid_body").WithMessage("rpc method expects no request body")
		}

		req := reflect.New(handlerType.In(1).Elem())
		err := json.Unmarshal(body, req.Interface())
		if err != nil {
			reqLogger.Entry().
				WithError(fmt.Errorf("error parsing request body: %w", err)).
				WithField("handler_duration", getDuration(startedAt)).
				Warn("request handled error")

			return nil, hand.New("invalid_body").WithMessage("body parsing error")
		}

		result = handlerValue.Call([]reflect.Value{reflect.ValueOf(ctx), req})
	} else {
		if m.expectsRequestBody {
			reqLogger.Entry().
				WithError(hand.New("request_body_expected")).
				WithField("handler_duration", getDuration(startedAt)).
				Warn("rpc request handled error")

			return nil, hand.New("invalid_body").WithMessage("rpc method expects a request body")
		}

		result = handlerValue.Call([]reflect.Value{reflect.ValueOf(ctx)})
	}

	reqLogger.Update(reqLogger.Entry().WithField("handler_duration", getDuration(startedAt)))

	resultErr := result[len(result)-1]

	if resultErr.IsNil() {
		reqLogger.Entry().Info("rpc request handled")

		return result[0].Interface(), nil
	}

	err, _ := resultErr.Interface().(error)

	reqLogger.Update(reqLogger.Entry().WithError(err))

	if handErr, ok := err.(hand.E); ok {
		if handErr.Err != nil {
			reqLogger.Update(reqLogger.Entry().WithField("err_cause", handErr.Err))
		}
		if handErr.Message != "" {
			reqLogger.Update(reqLogger.Entry().WithField("err_message", handErr.Message))
		}

		if handErr.Code == runtime.ErrCodeUnknown {
			reqLogger.Entry().Error("rpc request handled error")
		} else {
			reqLogger.Entry().Warn("rpc request handled error")
		}

		return nil, handErr
	}

	reqLogger.Entry().Error("rpc request unhandled error")

	return nil, hand.New(runtime.ErrCodeUnknown)
}

func getDuration(startedAt time.Time) float64 {
	return float64(time.Now().Sub(startedAt).Microseconds()*100) / 100000
}
