package finish

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/google/uuid"
)

type tripFinisherMock struct {
	finishFunc func(context.Context, string, time.Time) error
}

func (m *tripFinisherMock) Finish(
	ctx context.Context,
	tripID string,
	finishedAt time.Time,
) error {
	return m.finishFunc(ctx, tripID, finishedAt)
}

type tripGetterMock struct {
	getFunc func(context.Context, string) (model.Trip, error)
}

func (m *tripGetterMock) Get(
	ctx context.Context,
	tripID string,
) (model.Trip, error) {
	return m.getFunc(ctx, tripID)
}

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()
	internalErr := errors.New("database password or internal details")

	validTrip := model.Trip{
		ID:             tripID.String(),
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  10,
		StartLongitude: 20,
		EndLatitude:    30,
		EndLongitude:   40,
		Price:          100,
		Status:         "completed",
		StartedAt:      time.Now().Add(-time.Hour),
	}

	tests := []struct {
		name            string
		finishErr       error
		getErr          error
		trip            model.Trip
		wantStatus      int
		wantBodyContent string
		wantGetCalled   bool
	}{
		{
			name:            "success",
			trip:            validTrip,
			wantStatus:      http.StatusOK,
			wantBodyContent: "completed",
			wantGetCalled:   true,
		},
		{
			name:            "trip already completed",
			finishErr:       tripDomain.ErrAlreadyCompleted,
			wantStatus:      http.StatusConflict,
			wantBodyContent: "trip_already_completed",
		},
		{
			name:            "trip not found",
			finishErr:       tripDomain.ErrNotFound,
			wantStatus:      http.StatusNotFound,
			wantBodyContent: "not_found",
		},
		{
			name:            "internal error",
			finishErr:       internalErr,
			wantStatus:      http.StatusInternalServerError,
			wantBodyContent: "internal_error",
		},
		{
			name:            "get trip error after finishing",
			trip:            validTrip,
			getErr:          internalErr,
			wantStatus:      http.StatusInternalServerError,
			wantBodyContent: "internal_error",
			wantGetCalled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getCalled := false

			finisher := &tripFinisherMock{
				finishFunc: func(
					_ context.Context,
					gotTripID string,
					finishedAt time.Time,
				) error {
					if gotTripID != tripID.String() {
						t.Errorf("Finish() tripID = %q, want %q", gotTripID, tripID.String())
					}
					if finishedAt.IsZero() {
						t.Error("Finish() received zero finishedAt")
					}
					return tt.finishErr
				},
			}

			getter := &tripGetterMock{
				getFunc: func(
					_ context.Context,
					gotTripID string,
				) (model.Trip, error) {
					getCalled = true
					if gotTripID != tripID.String() {
						t.Errorf("Get() tripID = %q, want %q", gotTripID, tripID.String())
					}
					return tt.trip, tt.getErr
				},
			}

			handler := NewHandler(finisher, getter)
			req := httptest.NewRequest(http.MethodPost, "/trips/"+tripID.String()+"/finish", nil)
			rec := httptest.NewRecorder()

			handler.Handle(rec, req, api.TripId(tripID))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if !strings.Contains(rec.Body.String(), tt.wantBodyContent) {
				t.Errorf("response body %q does not contain %q", rec.Body.String(), tt.wantBodyContent)
			}

			if getCalled != tt.wantGetCalled {
				t.Errorf("getter called = %v, want %v", getCalled, tt.wantGetCalled)
			}

			if tt.wantStatus == http.StatusInternalServerError &&
				strings.Contains(rec.Body.String(), internalErr.Error()) {
				t.Error("response leaked internal error details")
			}
		})
	}
}
