package driver

import (
	"context"

	idriver "github.com/HMasataka/collision/domain/driver"
	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/errs"
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

func (d *lockerDriver) FetchTicketLock(ctx context.Context) (context.Context, context.CancelFunc, *errs.Error) {
	locked, unlock, err := d.locker.WithContext(context.Background(), d.fetchTicketsLock())
	if err != nil {
		return nil, nil, entity.ErrLockAcquisitionFailed.WithCause(err)
	}

	return locked, unlock, nil
}
