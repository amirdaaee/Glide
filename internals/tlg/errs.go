package tlg

import "fmt"

// UnexpectedTypeErrType is returned when a Telegram RPC value has the wrong type.
type UnexpectedTypeErrType struct {
	ExpectedType any
	GotType      any
}

// Error describes the expected and actual types.
func (e *UnexpectedTypeErrType) Error() string {
	return fmt.Sprintf("expected %T got %T", e.ExpectedType, e.GotType)
}
