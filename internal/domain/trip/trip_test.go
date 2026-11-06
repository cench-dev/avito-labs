package trip

import (
	"errors"
	"testing"
	"time"
)

func TestTrip_Finish(t *testing.T) {
	finishedAt := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		status      Status
		finishedAt  time.Time
		wantErr     error
		wantStatus  Status
		wantHasTime bool
	}{
		{
			name:        "active trip finishes successfully",
			status:      StatusActive,
			finishedAt:  finishedAt,
			wantStatus:  StatusCompleted,
			wantHasTime: true,
		},
		{
			name:       "completed trip cannot finish again",
			status:     StatusCompleted,
			finishedAt: finishedAt,
			wantErr:    ErrAlreadyCompleted,
			wantStatus: StatusCompleted,
		},
		{
			name:       "unknown status is rejected",
			status:     Status("unknown"),
			finishedAt: finishedAt,
			wantErr:    ErrInvalidTrip,
			wantStatus: Status("unknown"),
		},
		{
			name:       "zero finish time is rejected",
			status:     StatusActive,
			wantErr:    ErrInvalidTrip,
			wantStatus: StatusActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trip := Trip{Status: tt.status}

			err := trip.Finish(tt.finishedAt)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Finish() error = %v, want %v", err, tt.wantErr)
			}

			if trip.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", trip.Status, tt.wantStatus)
			}

			if got := trip.FinishedAt != nil; got != tt.wantHasTime {
				t.Errorf("FinishedAt present = %v, want %v", got, tt.wantHasTime)
			}

			if tt.wantHasTime && !trip.FinishedAt.Equal(finishedAt) {
				t.Errorf("FinishedAt = %v, want %v", *trip.FinishedAt, finishedAt)
			}
		})
	}
}

func TestTrip_Validate(t *testing.T) {
	finishedAt := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		trip    Trip
		wantErr error
	}{
		{
			name: "active trip is valid",
			trip: Trip{Status: StatusActive},
		},
		{
			name: "completed trip with finish time is valid",
			trip: Trip{Status: StatusCompleted, FinishedAt: &finishedAt},
		},
		{
			name:    "unknown status is invalid",
			trip:    Trip{Status: Status("unknown")},
			wantErr: ErrInvalidTrip,
		},
		{
			name:    "completed trip without finish time is invalid",
			trip:    Trip{Status: StatusCompleted},
			wantErr: ErrInvalidTrip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.trip.Validate()

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
