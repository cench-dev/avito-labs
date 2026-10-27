package get

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/model"
)

type tripGetterMock struct {
	getFunc func(context.Context, string) (model.Trip, error)
}

func (m *tripGetterMock) Get(ctx context.Context, tripID string) (model.Trip, error) {
	return m.getFunc(ctx, tripID)
}

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()
	startedAt := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		trip       model.Trip
		getErr     error
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			trip: model.Trip{
				ID:             tripID.String(),
				UserID:         uuid.NewString(),
				DriverID:       uuid.NewString(),
				StartLatitude:  10,
				StartLongitude: 20,
				EndLatitude:    30,
				EndLongitude:   40,
				Price:          100,
				Status:         "active",
				StartedAt:      startedAt,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "trip not found",
			getErr:     tripDomain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "internal error",
			getErr:     errors.New("secret database details"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &tripGetterMock{
				getFunc: func(_ context.Context, id string) (model.Trip, error) {
					if id != tripID.String() {
						t.Errorf("got trip ID %q, want %q", id, tripID.String())
					}
					return tt.trip, tt.getErr
				},
			}

			handler := NewHandler(mock)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)

			handler.Handle(rec, req, tripID)

			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantCode != "" {
				var problem api.Problem
				if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != tt.wantCode {
					t.Errorf("got code %q, want %q", problem.Code, tt.wantCode)
				}
			}

			if tt.name == "success" {
				if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
					t.Errorf("unexpected Content-Type: %q", rec.Header().Get("Content-Type"))
				}
			}

			if tt.name == "internal error" && strings.Contains(rec.Body.String(), "secret database details") {
				t.Error("response leaked internal error details")
			}
		})
	}
}
