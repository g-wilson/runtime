# Runtime

Opinionated framework for creating HTTP handlers written in Go that can be run in AWS Lambda and Amazon API Gateway HTTP APIs.

An example app can be found [here](https://github.com/g-wilson/runtime-helloworld).

## Components

### Handlers

The main abstraction this library provides is the Handler. It represents the "glue" layer between the execution environment (i.e. Lambda on HTTP API Gateway) and a generic Go application, provided by the developer.

It uses reflection to wrap ordinary Go functions (from the developer's application) to a generic Lambda-compatible handler function.

A JSON handler is provided. You provide a Go function which uses pointer struct input and return arguments, with JSON struct tags, plus a JSON Schema definition for validation.

### Middleware

Handlers are an interface type (much like net/http.Handler) which can be decorated by other Handlers, allowing for middleware-style functions.

### Hand

`hand` is an error type which represents an "error by design" - an outcome which is not the happy path but is _handled_ by the system as an expected behaviour.

When returned by a handler function, `hand` errors can be serialised (using the error response middlewares) into the response JSON. Therefore, an error should be _handled_ only if it is safe to return to clients. If you have debug data from errors, you should log them.

### Logging

A request logging middleware is provided, which uses Logrus due to its popularity and familiar semantics. This middlware not only logs request data, it also adds the logrus Entry to the context. This should be used within methods so that when your application writes log messages, you can have contextual data attached as fields automatically - such as the request ID, crucially!

## Example

See the [example repo](https://github.com/g-wilson/runtime-helloworld).
