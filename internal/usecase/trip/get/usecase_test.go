package get

import (
	"context"
	"errors"
	"testing"

	"github.com/cench-dev/avito-labs/internal/model"
	"go.uber.org/mock/gomock"
)

func TestUseCase_Get(t *testing.T) {
	tests := []struct {
		name    string
		trip    model.Trip
		repoErr error
		want    model.Trip
		wantErr error
	}{
		{
			name: "success",
			trip: model.Trip{ID: "trip-1", Status: "active"},
			want: model.Trip{ID: "trip-1", Status: "active"},
		},
		{
			name:    "repository error",
			repoErr: errors.New("database failed"),
			wantErr: errors.New("database failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)

			repo.EXPECT().
				GetTrip(gomock.Any(), "trip-1").
				Return(tt.trip, tt.repoErr)

			got, err := NewUseCase(repo).Get(context.Background(), "trip-1")

			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
