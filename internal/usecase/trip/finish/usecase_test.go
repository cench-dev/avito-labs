package finish

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/cench-dev/avito-labs/internal/model"
)

func TestUseCase_Finish(t *testing.T) {
	tests := []struct {
		name       string
		finishErr  error
		historyErr error
		wantErr    string
	}{
		{name: "success"},
		{
			name:      "finish repository error",
			finishErr: errors.New("finish failed"),
			wantErr:   "finish failed",
		},
		{
			name:       "history repository error",
			historyErr: errors.New("history failed"),
			wantErr:    "history failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			tx := NewMocktxManager(ctrl)
			metrics := NewMockbusinessMetrics(ctrl)

			finishedAt := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

			tx.EXPECT().
				Do(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})

			repo.EXPECT().
				GetTrip(gomock.Any(), "trip-1").
				Return(model.Trip{
					ID:     "trip-1",
					Status: "active",
				}, nil)

			repo.EXPECT().
				FinishTrip(gomock.Any(), "trip-1", finishedAt).
				Return(tt.finishErr)

			if tt.finishErr == nil {
				repo.EXPECT().
					CreateTripStatusHistory(
						gomock.Any(),
						model.TripStatusHistory{
							TripID:     "trip-1",
							FromStatus: stringPtr("active"),
							ToStatus:   "completed",
						},
					).
					Return(tt.historyErr)
			}

			if tt.wantErr == "" {
				metrics.EXPECT().
					RecordTripCompleted(gomock.Any()).
					Times(1)
			} else {
				metrics.EXPECT().
					RecordTripCompleted(gomock.Any()).
					Times(0)
			}

			err := NewUseCase(repo, tx, metrics).Finish(
				context.Background(),
				"trip-1",
				finishedAt,
			)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
