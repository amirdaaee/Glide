package service

import (
	"errors"

	"github.com/amirdaaee/Glide/internals/domain"
)

func getOrCreate[T any](
	get func() (*T, error),
	create func(*T) error,
	candidate *T,
) (*T, error) {
	existing, err := get()
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if err := create(candidate); err != nil {
		return nil, err
	}
	return candidate, nil
}
