# Runtime

Opinionated framework for creating JSON-based RPC methods written in Go that can be run in AWS Lambda and Amazon API Gateway HTTP APIs.

An example app can be found [here](https://github.com/g-wilson/runtime-helloworld).

## Components

### Method

The main abstraction this library provides is the RPC Method. It represents the "glue" layer between the execution environment (i.e. Lambda on HTTP API Gateway) and a generic Go application, provided by the developer.

It uses reflection to link ordinary Go functions (from the developer's application) to an external JSON-based interface. Provide a method name `string`, a handler function `interface{}`, and a JSON-Schema for argument validation to get going.

Internally, it provides several utilities expected of a modern application such as a pattern for error responses, a contextual logger (logs request details but can also be used by the application), and an abstraction for authentication state.

### Hand

`hand` is an error type which represents an "error by design" - an outcome which is not the happy path but is _handled_ by the system as an expected behaviour.

When returned by an RPC method, `hand` errors are serialised into the response JSON. Therefore, an error should be _handled_ only if it is safe to return to clients. If you have debug data from errors, you should log them.

An additional benefit of this approach is that the RPC Client can coerce a JSON response body and test for conformance of the `hand` type - which means error propagation between RPC services is taken care of.

### Logging

Logging is designed to be useful but extensible. Logrus is used due to its popularity and semantics.

All invocations of RPC methods are logged. And the request logger understands the `hand` error type, so you can have useful output with little up-front effrort.

The Go context within a method is provided with a context-aware logger. This should be used within methods so that when your application writes log messages, you can have contextual data attached as fields automatically - such as the request ID, crucially!

### Authentication

JWT authentication from the execution environment can be parsed and made available to the application using a "Identity Provider" callback function on the Method. This function can be used to convert the generic map of claims into a more useful application-specific type.

There is no built-in authentication or token validation to the RPC Method handling itself. It is assumed that you'd run any automatic authentication provided by the execution environment, for example JWT Authorizers in AWS API Gateway. However, there is a JWT validation utility which is designed for non-production contexts.
