// Package bot provides error types and helpers for the bot module.
package bot

import "fmt"

// BotError represents an error in the bot package with an optional wrapped error.
type BotError struct {
	Msg string
	Err error
}

// Error returns the error message for BotError, including any wrapped error.
func (e *BotError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

// Unwrap returns the wrapped error, if any, for compatibility with errors.Unwrap.
func (e *BotError) Unwrap() error {
	return e.Err
}

// NewBotError creates a new BotError with the given message and wrapped error.
func NewBotError(msg string, err error) *BotError {
	return &BotError{Msg: msg, Err: err}
}
