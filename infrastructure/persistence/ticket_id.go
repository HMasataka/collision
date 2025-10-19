package persistence

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/errs"
	"github.com/redis/rueidis"
)

type ticketIDRepository struct {
	client rueidis.Client
}

func NewTicketIDRepository(
	client rueidis.Client,
) repository.TicketIDRepository {
	return &ticketIDRepository{
		client: client,
	}
}

func (r *ticketIDRepository) TicketIDKey() string {
	return "ticket:ids"
}

func (r *ticketIDRepository) GetAllTicketIDs(ctx context.Context, limit int64) ([]string, *errs.Error) {
	query := r.client.B().Srandmember().Key(r.TicketIDKey()).Count(limit).Build()

	resp := r.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}

		return nil, entity.ErrIndexGetFailed.WithCause(err)
	}

	allTicketIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, entity.ErrIndexDecodeFailed.WithCause(err)
	}

	return allTicketIDs, nil
}
