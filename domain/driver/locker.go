package driver

import (
	"context"
)

type LockerDriver interface {
	FetchTicketLock(ctx context.Context) (context.Context, context.CancelFunc, error)
}
