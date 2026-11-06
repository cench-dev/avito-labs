package trip

import "errors"

var (
	ErrNotFound         = errors.New("trip not found")
	ErrAlreadyCompleted = errors.New("trip already completed")
	ErrDriverBusy       = errors.New("driver is already busy")
)
