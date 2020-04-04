package asyncservice

// TODO: SUPER PROTOTYPEY DO NOT USE

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/aws/aws-lambda-go/events"
)

type LambdaSQSHandler func(context.Context, events.SQSEvent) error

func UnmarshalLambdaEvent(body string, dest interface{}) error {
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

func HandleSQS(handler interface{}) LambdaSQSHandler {
	// TODO: validate handler is function with correct args
	expectedType := reflect.TypeOf(handler).In(1)

	return func(ctx context.Context, sqsEvent events.SQSEvent) error {
		if len(sqsEvent.Records) != 1 {
			return errors.New("sqs worker expects exactly one record - check batch size")
		}

		sqsMsg := sqsEvent.Records[0]

		// if the handler expects an SQS type then handle it now
		if expectedType == reflect.TypeOf(sqsEvent) {
			return invoke(ctx, handler, sqsMsg)
		}

		// check nested SNS
		if sqsMsg.EventSource == "aws:sns" {
			snsEvt := &events.SNSEvent{}
			err := UnmarshalLambdaEvent(sqsMsg.Body, snsEvt)
			if err == nil {
				return err
			}
			if len(snsEvt.Records) != 1 {
				return errors.New("sqs worker expects exactly one sns event record per sqs record")
			}

			snsMsg := snsEvt.Records[0]

			// if the handler expects an SNS type then handle it now
			if expectedType == reflect.TypeOf(snsEvt) {
				return invoke(ctx, handler, snsMsg)
			}

			// check nested S3
			if snsMsg.EventSource == "aws:s3" {
				s3Evt := &events.S3Event{}
				err := UnmarshalLambdaEvent(snsMsg.SNS.Message, s3Evt)
				if err == nil {
					return err
				}
				if expectedType == reflect.TypeOf(s3Evt) {
					return invoke(ctx, handler, s3Evt)
				}
			}
		}

		return fmt.Errorf("handler expected event of type %s", expectedType.Name())
	}
}

func invoke(ctx context.Context, handler interface{}, msg interface{}) error {
	hv := reflect.ValueOf(handler)

	res := hv.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(msg),
	})

	if err := res[len(res)-1]; !err.IsNil() {
		if err, ok := err.Interface().(error); ok {
			log.Println(err)
			return err
		}

		err := errors.New("method errored but error was of invalid type")
		return err
	}

	return nil
}
