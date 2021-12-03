package http

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

type Handler interface {
	Handle(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error)
}

type HandlerFunc func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error)

func (f HandlerFunc) Handle(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	return f(ctx, event)
}

type Middleware func(Handler) Handler

func WithMiddleware(h Handler, mwares ...Middleware) Handler {
	for _, mw := range mwares {
		h = mw(h)
	}
	return h
}
