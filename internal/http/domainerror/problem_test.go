package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "trip not found",
			err:        tripDomain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "trip already completed",
			err:        tripDomain.ErrAlreadyCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_already_completed",
		},
		{
			name:       "driver busy",
			err:        tripDomain.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "internal error hides details",
			err:        errors.New("secret database connection string"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			Write(rec, req, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}

			var problem api.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if problem.Code != tt.wantCode {
				t.Errorf("code: got %q, want %q", problem.Code, tt.wantCode)
			}

			if tt.name == "internal error hides details" &&
				contains(rec.Body.String(), "secret database connection string") {
				t.Error("response leaked internal error details")
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && index(s, substr) >= 0
}

func index(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
