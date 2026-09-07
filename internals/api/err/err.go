package err

import "fmt"

// HttpError is an HTTP error with a status code and message.
type HttpError struct {
	Status  int
	Message string
}

// Error returns the HTTP error message.
func (e *HttpError) Error() string {
	return e.Message
}

var (
	// ErrBadRequest is a 400 invalid-request error.
	ErrBadRequest = &HttpError{Status: 400, Message: "invalid request"}
	// ErrUnauthorized is a 403 forbidden error.
	ErrUnauthorized = &HttpError{Status: 403, Message: "forbidden"}
	// ErrNotFound is a 404 missing-resource error.
	ErrNotFound = &HttpError{Status: 404, Message: "resource not found"}
)

// NewTooManyRequestsError returns a 429 error wrapping err.
func NewTooManyRequestsError(err error) *HttpError {
	return &HttpError{Status: 429, Message: fmt.Sprintf("too many requests. %s", err.Error())}
}

// NewBadrequestError returns a 400 error wrapping err.
func NewBadrequestError(err error) *HttpError {
	return &HttpError{Status: 400, Message: fmt.Sprintf("bad request. %s", err.Error())}
}
