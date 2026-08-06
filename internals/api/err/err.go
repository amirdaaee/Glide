package err

import "fmt"

type HttpError struct {
	Status  int
	Message string
}

func (e *HttpError) Error() string {
	return e.Message
}

var (
	ErrBadRequest   = &HttpError{Status: 400, Message: "invalid request"}
	ErrUnauthorized = &HttpError{Status: 403, Message: "forbidden"}
	ErrNotFound     = &HttpError{Status: 404, Message: "resource not found"}
)

func NewTooManyRequestsError(err error) *HttpError {
	return &HttpError{Status: 429, Message: fmt.Sprintf("too many requests. %s", err.Error())}
}
func NewBadrequestError(err error) *HttpError {
	return &HttpError{Status: 400, Message: fmt.Sprintf("bad request. %s", err.Error())}
}
