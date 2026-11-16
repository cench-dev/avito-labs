package positions

import "context"

type pushGateway interface {
	Send(ctx context.Context, tripID, driverID string) error
}
