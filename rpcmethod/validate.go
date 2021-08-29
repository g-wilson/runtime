package rpcmethod

import (
	"errors"
	"fmt"
	"reflect"
)

// validateMethod analyses a Method for requirements before it can be served
// valid handler functions:
// func(ctx context.Context, request *T) (response *T, err error)
// func(ctx context.Context, request *T) (err error)
// func(ctx context.Context) (response *T, err error)
// func(ctx context.Context) (err error)
func validate(m *Method) (hasReqBody, hasResBody bool, err error) {
	handlerValue := reflect.ValueOf(m.handler)
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

		if m.compiledSchema == nil {
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
