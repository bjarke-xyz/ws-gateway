package domain

import "errors"

var (
	// ErrNotFound is returned when something is not found
	ErrNotFound = errors.New("item not found")
	ErrConflict = errors.New("item already exists")
)
