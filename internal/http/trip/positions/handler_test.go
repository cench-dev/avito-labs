package positions

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
	"go.uber.org/mock/gomock"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/model"
)

const (
	invalidRequestCode   = "invalid_request"
	notFoundPositionBody = `{"coordinates":{"latitude":59,"longitude":30},"recorded_at":"2026-01-02T10:00:00Z"}`
	invalidLatitudeBody  = `{"coordinates":{"latitude":91,"longitude":30},"recorded_at":"2026-01-02T10:00:00Z"}`
	invalidLongitudeBody = `{"coordinates":{"latitude":59,"longitude":181},"recorded_at":"2026-01-02T10:00:00Z"}`
)

func TestHandler_Create(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		body       string
		createID   int64
		createErr  error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "success",
			body:       notFoundPositionBody,
			createID:   42,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid latitude",
			body:       invalidLatitudeBody,
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
		{
			name:       "invalid longitude",
			body:       invalidLongitudeBody,
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
		{
			name:       "invalid json",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
		{
			name:       "trip not found",
			body:       notFoundPositionBody,
			createErr:  tripDomain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "trip completed",
			body:       notFoundPositionBody,
			createErr:  tripDomain.ErrAlreadyCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "internal error",
			body:       notFoundPositionBody,
			createErr:  errors.New("secret database details"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mock := NewMockPositionUseCase(ctrl)

			if tt.wantCode == "" || tt.createErr != nil {
				if tt.wantStatus == http.StatusCreated || tt.createErr != nil {
					mock.EXPECT().
						Create(gomock.Any(), gomock.Any()).
						DoAndReturn(func(
							_ context.Context,
							position model.TripPosition,
						) (int64, error) {
							if position.TripID != tripID.String() {
								t.Errorf("got trip ID %q, want %q", position.TripID, tripID.String())
							}
							return tt.createID, tt.createErr
						})
				}
			}

			handler := NewHandler(mock, testPositionCanceler{})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/trips/"+tripID.String()+"/positions",
				strings.NewReader(tt.body),
			)

			handler.Create(rec, req, tripID)

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

			if tt.wantStatus == http.StatusCreated {
				var response api.TripPosition
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Id != tt.createID {
					t.Errorf("got ID %d, want %d", response.Id, tt.createID)
				}
				if response.TripId != tripID {
					t.Errorf("got trip ID %q, want %q", response.TripId, tripID)
				}
				if !response.RecordedAt.Equal(recordedAt) {
					t.Errorf("got recorded_at %v, want %v", response.RecordedAt, recordedAt)
				}
			}

			if tt.wantStatus == http.StatusInternalServerError &&
				strings.Contains(rec.Body.String(), "secret database details") {
				t.Error("response leaked internal error details")
			}
		})
	}
}

func TestHandler_List(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		positions  []model.TripPosition
		getErr     error
		wantStatus int
		wantCode   string
		wantCount  int
	}{
		{
			name: "success",
			positions: []model.TripPosition{
				{
					ID:         1,
					TripID:     tripID.String(),
					Latitude:   59.9398,
					Longitude:  30.3146,
					RecordedAt: recordedAt,
				},
			},
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "empty positions",
			positions:  []model.TripPosition{},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "trip not found",
			getErr:     tripDomain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
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
			ctrl := gomock.NewController(t)
			mock := NewMockPositionUseCase(ctrl)

			mock.EXPECT().
				Get(gomock.Any(), tripID.String()).
				Return(tt.positions, tt.getErr).
				Times(1)

			handler := NewHandler(mock, testPositionCanceler{})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/trips/"+tripID.String()+"/positions",
				nil,
			)

			handler.List(rec, req, tripID)

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

			if tt.wantStatus == http.StatusOK {
				var response api.TripPositionsResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Positions) != tt.wantCount {
					t.Errorf("got %d positions, want %d", len(response.Positions), tt.wantCount)
				}
			}
		})
	}
}

type testPositionCanceler struct{}

func (testPositionCanceler) Cancel(string) {}
