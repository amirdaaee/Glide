package domain

import "errors"

var (
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists is returned when creating a record that already exists.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidTransition is returned when a status or step change is illegal.
	ErrInvalidTransition = errors.New("invalid transition")
)
