package finish

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	"github.com/cench-dev/avito-labs/internal/model"
)

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
			wantBodyContent: "trip_completed",
		},
		{
			name:            "trip not found",
			finishErr:       tripDomain.ErrNotFound,
			wantStatus:      http.StatusNotFound,
			wantBodyContent: "trip_not_found",
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
			ctrl := gomock.NewController(t)

			finisher := NewMockTripFinisher(ctrl)
			getter := NewMockTripGetter(ctrl)

			finisher.EXPECT().
				Finish(gomock.Any(), tripID.String(), gomock.Any()).
				DoAndReturn(func(
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
				}).
				Times(1)

			if tt.wantGetCalled {
				getter.EXPECT().
					Get(gomock.Any(), tripID.String()).
					DoAndReturn(func(
						_ context.Context,
						gotTripID string,
					) (model.Trip, error) {
						if gotTripID != tripID.String() {
							t.Errorf("Get() tripID = %q, want %q", gotTripID, tripID.String())
						}
						return tt.trip, tt.getErr
					}).
					Times(1)
			}

			handler := NewHandler(finisher, getter)
			req := httptest.NewRequest(
				http.MethodPost,
				"/trips/"+tripID.String()+"/finish",
				nil,
			)
			rec := httptest.NewRecorder()

			handler.Handle(rec, req, tripID)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body: %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if !strings.Contains(rec.Body.String(), tt.wantBodyContent) {
				t.Errorf(
					"response body %q does not contain %q",
					rec.Body.String(),
					tt.wantBodyContent,
				)
			}

			if tt.wantStatus == http.StatusInternalServerError &&
				strings.Contains(rec.Body.String(), internalErr.Error()) {
				t.Error("response leaked internal error details")
			}
		})
	}
}
