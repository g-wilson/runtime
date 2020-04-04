package hand

import (
	"fmt"
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

func New(code string) E {
	return E{Code: code}
}

func Wrap(code string, err error) E {
	return E{Code: code, Err: err}
}

func Errorf(msg string, values ...interface{}) E {
	return New(fmt.Sprintf(msg, values...))
}
