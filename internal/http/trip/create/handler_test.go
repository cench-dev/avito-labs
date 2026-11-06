package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
)

func TestHandler_Handle(t *testing.T) {
	userID := uuid.New()
	driverID := uuid.New()

	validBody, err := json.Marshal(api.TripData{
		UserId:   userID,
		DriverId: driverID,
		StartPoint: api.Coordinates{
			Latitude:  10,
			Longitude: 20,
		},
		EndPoint: api.Coordinates{
			Latitude:  30,
			Longitude: 40,
		},
		Price: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
		wantCode   string
		wantCreate bool
	}{
		{
			name:       "success",
			body:       string(validBody),
			wantStatus: http.StatusCreated,
			wantCreate: true,
		},
		{
			name:       "invalid JSON",
			body:       "{",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "driver busy",
			body:       string(validBody),
			createErr:  tripDomain.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantCreate: true,
		},
		{
			name:       "internal error hides details",
			body:       string(validBody),
			createErr:  errors.New("secret database details"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mock := NewMockTripCreator(ctrl)

			if tt.wantCreate {
				mock.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(tt.createErr).
					Times(1)
			}

			handler := NewHandler(mock)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/trips",
				bytes.NewBufferString(tt.body),
			)

			handler.Handle(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d", rec.Code, tt.wantStatus)
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
				if rec.Header().Get("Location") == "" {
					t.Error("expected Location header")
				}
				if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
					t.Error("expected application/json content type")
				}
			}

			if tt.name == "internal error hides details" &&
				strings.Contains(rec.Body.String(), "secret database details") {
				t.Error("response leaked internal error details")
			}
		})
	}
}
