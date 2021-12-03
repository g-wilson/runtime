package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/g-wilson/runtime/ctxlog"
	"github.com/g-wilson/runtime/hand"

	"github.com/aws/aws-lambda-go/events"
	"github.com/sirupsen/logrus"
)

func CreateRequestLogger(l *logrus.Entry) Middleware {
	return func(h Handler) Handler {
		return HandlerFunc(func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
			ctx = ctxlog.SetContext(ctx, l.WithField("apig_request_id", event.RequestContext.RequestID))
			reqLogger := ctxlog.FromContext(ctx)

			startedAt := time.Now()

			res, err := h.Handle(ctx, event)

			reqLogger.Update(
				reqLogger.Entry().WithField("handler_duration", getDuration(startedAt)),
			)

			if err == nil {
				reqLogger.Entry().Info("request: handled success")

				return res, err
			}

			reqLogger.Update(reqLogger.Entry().WithError(err))

			handErr, ok := err.(hand.E)
			if !ok {
				reqLogger.Entry().Error("request: unhandled error")

				return res, err
			}

			if handErr.Err != nil {
				reqLogger.Update(reqLogger.Entry().WithField("err_cause", handErr.Err))
			}
			if handErr.Message != "" {
				reqLogger.Update(reqLogger.Entry().WithField("err_message", handErr.Message))
			}

			if handErr.Code == hand.ErrCodeUnknown {
				reqLogger.Entry().Error("request: handled error")
			} else {
				reqLogger.Entry().Warn("request: handled error")
			}

			return res, handErr
		})
	}
}

func getDuration(startedAt time.Time) float64 {
	return float64(time.Now().Sub(startedAt).Microseconds()*100) / 100000
}

func JSONErrorHandler(h Handler) Handler {
	return HandlerFunc(func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
		res, err := h.Handle(ctx, event)
		if err == nil {
			return res, err
		}

		var serialisedError []byte
		var status int

		if handErr, ok := err.(hand.E); ok {
			status = handErr.HTTPStatus()
			serialisedError, _ = json.Marshal(handErr)
		} else {
			status = http.StatusInternalServerError
			serialisedError, _ = json.Marshal(hand.New(hand.ErrCodeUnknown))
		}

		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       string(serialisedError),
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
		}, nil
	})
}

func TextErrorHandler(h Handler) Handler {
	return HandlerFunc(func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
		res, err := h.Handle(ctx, event)
		if err == nil {
			return res, err
		}

		var serialisedError string
		var status int

		if handErr, ok := err.(hand.E); ok {
			status = handErr.HTTPStatus()
			serialisedError = handErr.Error()
		} else {
			status = http.StatusInternalServerError
			serialisedError = hand.New(hand.ErrCodeUnknown).Error()
		}

		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       serialisedError,
			Headers: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
		}, nil
	})
}
