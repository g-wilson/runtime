package hand

import (
	"fmt"
	"net/http"
)

type M map[string]interface{}

type E struct {
	Code    string `json:"code"`
	Err     error  `json:"-"`
	Message string `json:"message,omitempty"`
	Meta    M      `json:"meta,omitempty"`
}

func (h E) Error() string {
	return h.Code
}

func (h E) WithMessage(msg string) E {
	return E{
		Code:    h.Code,
		Err:     h.Err,
		Meta:    h.Meta,
		Message: msg,
	}
}

func (h E) WithMeta(meta M) E {
	return E{
		Code:    h.Code,
		Err:     h.Err,
		Message: h.Message,
		Meta:    meta,
	}
}

func (h E) HTTPStatus() int {
	var status int

	switch h.Code {
	case ErrCodeBadRequest:
		fallthrough
	case ErrCodeInvalidBody:
		fallthrough
	case ErrCodeSchemaFailure:
		fallthrough
	case ErrCodeMissingBody:
		status = http.StatusBadRequest

	case ErrCodeForbidden:
		status = http.StatusForbidden

	case ErrCodeNoAuthentication:
		fallthrough
	case ErrCodeInvalidAuthentication:
		status = http.StatusUnauthorized

	default:
		status = http.StatusInternalServerError
	}

	return status
}

func New(code string) E {
	return E{Code: code}
}

func Wrap(code string, err error) E {
	return E{Code: code, Err: err}
}

func Errorf(msg string, values ...interface{}) E {
	return New(fmt.Sprintf(msg, values...))
}

func Matches(err error, comparator E) bool {
	handErr, ok := err.(E)
	if !ok {
		return false
	}
	return handErr.Code == comparator.Code
}
