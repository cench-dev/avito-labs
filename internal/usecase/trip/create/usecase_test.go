package create

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/cench-dev/avito-labs/internal/model"
)

func TestUseCase_Create(t *testing.T) {
	tests := []struct {
		name             string
		repoCreateErr    error
		historyCreateErr error
		txErr            error
		wantErr          error
	}{
		{
			name: "success",
		},
		{
			name:          "repository error",
			repoCreateErr: errors.New("insert trip failed"),
			wantErr:       errors.New("insert trip failed"),
		},
		{
			name:             "history error",
			historyCreateErr: errors.New("insert history failed"),
			wantErr:          errors.New("insert history failed"),
		},
		{
			name:    "transaction error",
			txErr:   errors.New("transaction failed"),
			wantErr: errors.New("transaction failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			tx := NewMocktxManager(ctrl)

			trip := model.Trip{
				ID:     "trip-1",
				Status: "active",
			}

			tx.EXPECT().
				Do(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					if tt.txErr != nil {
						return tt.txErr
					}
					return fn(ctx)
				})

			if tt.repoCreateErr != nil || tt.name == "success" ||
				tt.historyCreateErr != nil {
				repo.EXPECT().
					CreateTrip(gomock.Any(), trip).
					Return(tt.repoCreateErr)
			}

			if tt.repoCreateErr == nil && tt.txErr == nil {
				repo.EXPECT().
					CreateTripStatusHistory(gomock.Any(), gomock.Any()).
					Return(tt.historyCreateErr)
			}

			err := NewUseCase(repo, tx).Create(context.Background(), trip)

			if tt.wantErr == nil && err != nil {
				t.Fatalf("Create() error = %v, want nil", err)
			}
			if tt.wantErr != nil && (err == nil || err.Error() != tt.wantErr.Error()) {
				t.Fatalf("Create() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
