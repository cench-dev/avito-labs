package trip

import (
	"errors"
	"time"
)

var ErrInvalidTrip = errors.New("invalid trip")

type Trip struct {
	Status     Status
	FinishedAt *time.Time
}

func (t *Trip) Finish(finishedAt time.Time) error {
	if t.Status == StatusCompleted {
		return ErrAlreadyCompleted
	}

	if t.Status != StatusActive {
		return ErrInvalidTrip
	}

	if finishedAt.IsZero() {
		return ErrInvalidTrip
	}

	t.Status = StatusCompleted
	t.FinishedAt = &finishedAt

	return nil
}

func (t Trip) Validate() error {
	if t.Status != StatusActive && t.Status != StatusCompleted {
		return ErrInvalidTrip
	}

	if t.Status == StatusCompleted && t.FinishedAt == nil {
		return ErrInvalidTrip
	}

	return nil
}
