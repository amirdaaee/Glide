package tlg

import "fmt"

type UnexpectedTypeErrType struct {
	ExpectedType any
	GotType      any
}

func (e *UnexpectedTypeErrType) Error() string {
	return fmt.Sprintf("expected %T got %T", e.ExpectedType, e.GotType)
}
