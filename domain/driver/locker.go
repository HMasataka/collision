package driver

import (
	"context"

	"github.com/HMasataka/errs"
)

type LockerDriver interface {
	FetchTicketLock(ctx context.Context) (context.Context, context.CancelFunc, *errs.Error)
}
