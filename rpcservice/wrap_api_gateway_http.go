package rpcservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaAPIGatewayHandler is the expected function signature for AWS Lambda functions consuming events from API Gateway
type LambdaAPIGatewayHandler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error)

// WrapAPIGatewayHTTP wraps the service methods and returns a Lambda compatible handler function for HTTP API Gateway requests
func (s *Service) WrapAPIGatewayHTTP() LambdaAPIGatewayHandler {
	return func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (res events.APIGatewayProxyResponse, err error) {
		ctx = logger.SetContext(ctx, s.Logger.WithField("request_id", event.RequestContext.RequestID))
		reqLogger := logger.FromContext(ctx)

		if s.IdentityProvider != nil {
			authdata := event.RequestContext.Authorizer.JWT
			atclaims := AccessTokenClaims{}
			atclaims["scope"] = strings.Join(authdata.Scopes, " ")

			for key, val := range authdata.Claims {
				// apig jwt authorizer coerces audience to a string, split it for better compatibility
				if key == "aud" {
					atclaims["aud"] = strings.Split(strings.Trim(val, "[]"), " ")
				} else {
					atclaims[key] = val
				}
			}

			s.IdentityProvider(ctx, atclaims)
		}

		if len(event.PathParameters) < 1 {
			reqLogger.Entry().WithError(errors.New("wrap http api gateway: no path parameters found")).Error("request failed")
			return apiGatewayErrorResponse(hand.New("method_not_found")), nil
		}
		methodName, ok := event.PathParameters["method"]
		if !ok {
			reqLogger.Entry().WithError(errors.New("wrap http api gateway: method path parameter not found")).Error("request failed")
			return apiGatewayErrorResponse(hand.New("method_not_found")), nil
		}

		handler, ok := s.GetMethod(methodName)
		if !ok {
			reqLogger.Entry().WithError(fmt.Errorf("wrap http api gateway: method with name %s not found", methodName)).Error("request failed")
			return apiGatewayErrorResponse(hand.New("method_not_found")), nil
		}

		result, err := handler.Invoke(ctx, []byte(event.Body))
		if err != nil {
			return apiGatewayErrorResponse(err), nil
		}

		if result == nil {
			return events.APIGatewayProxyResponse{
				StatusCode:      http.StatusNoContent,
				Body:            "",
				IsBase64Encoded: false,
			}, nil
		}

		resBytes, err := json.Marshal(result)
		if err != nil {
			reqLogger.Entry().WithError(fmt.Errorf("wrap http api gateway: encoding response body failed: %w", err)).Error("request failed")
			return apiGatewayErrorResponse(err), nil
		}

		return events.APIGatewayProxyResponse{
			StatusCode:      http.StatusOK,
			Body:            string(resBytes),
			IsBase64Encoded: false,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
		}, nil
	}
}

func apiGatewayErrorResponse(err error) events.APIGatewayProxyResponse {
	var res []byte
	var status int

	if handErr, ok := err.(hand.E); ok {
		switch handErr.Code {
		case runtime.ErrCodeBadRequest:
			fallthrough
		case runtime.ErrCodeInvalidBody:
			fallthrough
		case runtime.ErrCodeSchemaFailure:
			fallthrough
		case runtime.ErrCodeMissingBody:
			status = http.StatusBadRequest

		case runtime.ErrCodeForbidden:
			status = http.StatusForbidden

		case runtime.ErrCodeNoAuthentication:
			fallthrough
		case runtime.ErrCodeInvalidAuthentication:
			status = http.StatusUnauthorized

		default:
			status = http.StatusInternalServerError
		}

		res, _ = json.Marshal(handErr)
	} else {
		status = http.StatusInternalServerError
		res, _ = json.Marshal(hand.New(runtime.ErrCodeUnknown))
	}

	return events.APIGatewayProxyResponse{
		StatusCode:      status,
		Body:            string(res),
		IsBase64Encoded: false,
		Headers: map[string]string{
			"Content-Type": "application/json; charset=utf-8",
		},
	}
}
