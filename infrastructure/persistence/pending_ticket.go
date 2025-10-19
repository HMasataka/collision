package persistence

import (
	"context"
	"strconv"
	"time"

	"github.com/HMasataka/collision/domain/driver"
	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/errs"
	"github.com/redis/rueidis"
)

type pendingTicketRepository struct {
	client       rueidis.Client
	lockerDriver driver.LockerDriver
}

func NewPendingTicketRepository(
	client rueidis.Client,
	lockerDriver driver.LockerDriver,
) repository.PendingTicketRepository {
	return &pendingTicketRepository{
		client:       client,
		lockerDriver: lockerDriver,
	}
}

func (r *pendingTicketRepository) PendingTicketKey() string {
	return "pendingTicketIDs"
}

func (r *pendingTicketRepository) GetPendingTicketIDs(ctx context.Context) ([]string, *errs.Error) {
	rangeMin := strconv.FormatInt(time.Now().Add(-defaultPendingReleaseTimeout).Unix(), 10)
	rangeMax := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)

	query := r.client.B().Zrangebyscore().Key(r.PendingTicketKey()).Min(rangeMin).Max(rangeMax).Build()

	resp := r.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, entity.ErrPendingTicketGetFailed.WithCause(err)
	}

	pendingTicketIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, entity.ErrIndexDecodeFailed.WithCause(err)
	}

	return pendingTicketIDs, nil
}

func (r *pendingTicketRepository) InsertPendingTicket(ctx context.Context, ticketIDs []string) *errs.Error {
	score := float64(time.Now().Unix())

	query := r.client.B().Zadd().Key(r.PendingTicketKey()).ScoreMember()
	for _, ticketID := range ticketIDs {
		query = query.ScoreMember(score, ticketID)
	}

	resp := r.client.Do(ctx, query.Build())
	if err := resp.Error(); err != nil {
		return entity.ErrPendingTicketSetFailed.WithCause(err)
	}

	return nil
}

func (r *pendingTicketRepository) ReleaseTickets(ctx context.Context, ticketIDs []string) *errs.Error {
	lockedCtx, unlock, err := r.lockerDriver.FetchTicketLock(ctx)
	if err != nil {
		return entity.ErrLockAcquisitionFailed.WithCause(err)
	}
	defer unlock()

	query := r.client.B().Zrem().Key(r.PendingTicketKey()).Member(ticketIDs...).Build()

	resp := r.client.Do(lockedCtx, query)
	if err := resp.Error(); err != nil {
		return entity.ErrPendingTicketReleaseFailed.WithCause(err)
	}

	return nil
}
