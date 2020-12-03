package rpcservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"

	"github.com/aws/aws-lambda-go/events"
	"golang.org/x/sync/errgroup"
)

// LambdaSQSHandler is the expected function signature for AWS Lambda functions consuming events from SQS
type LambdaSQSHandler func(context.Context, events.SQSEvent) error

// WrapSQSHandler wraps the service methods and returns a Lambda compatible handler function for invoking one RPC method on a queue
func (s *Service) WrapSQSHandler(methodName string) LambdaSQSHandler {
	return func(ctx context.Context, sqsEvent events.SQSEvent) error {
		reqLogger := logger.FromContext(ctx)

		handler, ok := s.GetMethod(methodName)
		if !ok {
			reqLogger.Entry().WithError(fmt.Errorf("wrap sqs handler: method with name %s not found", methodName)).Error("invocation failed")
			return hand.New("method_not_found")
		}

		g, gctx := errgroup.WithContext(ctx)

		for _, msg := range sqsEvent.Records {
			g.Go(func() (err error) {
				gctx = logger.SetContext(gctx, s.Logger.WithField("sqs_msg_id", msg.MessageId))
				msgLogger := logger.FromContext(gctx)

				for _, fn := range s.ContextProviders {
					gctx = fn(gctx)
				}

				result, err := handler.Invoke(ctx, []byte(msg.Body))
				if err != nil {
					return
				}
				if result != nil {
					msgLogger.Entry().WithField("result", result).Info("invocation result")
				}

				return
			})
		}

		return g.Wait()
	}
}

func unmarshalBody(body string, dest interface{}) error {
	destType := reflect.TypeOf(dest)

	if destType.Kind() != reflect.Ptr {
		return errors.New("dest must be a pointer")
	}

	err := json.Unmarshal([]byte(body), reflect.ValueOf(dest).Interface())
	if err != nil {
		return err
	}

	return nil
}
