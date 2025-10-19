package driver

import (
	"context"

	idriver "github.com/HMasataka/collision/domain/driver"
	"github.com/redis/rueidis/rueidislock"
)

type lockerDriver struct {
	locker rueidislock.Locker
}

func (d *lockerDriver) fetchTicketsLock() string {
	return "fetchTicketsLock"
}

func NewLockerDriver(locker rueidislock.Locker) idriver.LockerDriver {
	return &lockerDriver{
		locker: locker,
	}
}

func (d *lockerDriver) FetchTicketLock(ctx context.Context) (context.Context, context.CancelFunc, error) {
	return d.locker.WithContext(context.Background(), d.fetchTicketsLock())
}
