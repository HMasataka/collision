package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/errs"
	"github.com/redis/rueidis"
)

const (
	defaultPendingReleaseTimeout = 1 * time.Minute
)

type ticketRepository struct {
	client rueidis.Client
}

func NewTicketRepository(
	client rueidis.Client,
) repository.TicketRepository {
	return &ticketRepository{
		client: client,
	}
}

func (r *ticketRepository) TicketDataKey(ticketID string) string {
	return ticketID
}

func (r *ticketRepository) ticketIDFromRedisKey(key string) string {
	return key
}

func (r *ticketRepository) GetTickets(ctx context.Context, ticketIDs []string) (entity.Tickets, []string, *errs.Error) {
	keys := make([]string, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		keys[i] = r.TicketDataKey(ticketID)
	}

	m, err := rueidis.MGet(r.client, ctx, keys)
	if err != nil {
		return nil, nil, entity.ErrTicketGetFailed.WithCause(err)
	}

	tickets := make(entity.Tickets, 0, len(keys))
	var ticketIDsNotFound []string

	for key, resp := range m {
		if err := resp.Error(); err != nil {
			if rueidis.IsRedisNil(err) {
				ticketIDsNotFound = append(ticketIDsNotFound, r.ticketIDFromRedisKey(key))
				continue
			}
			return nil, nil, entity.ErrTicketGetFailed.WithCause(err)
		}

		data, err := resp.AsBytes()
		if err != nil {
			return nil, nil, entity.ErrTicketGetFailed.WithCause(err)
		}

		var ticket entity.Ticket

		if err := json.Unmarshal(data, &ticket); err != nil {
			return nil, nil, entity.ErrTicketUnmarshalFailed.WithCause(err)
		}

		tickets = append(tickets, &ticket)
	}

	return tickets, ticketIDsNotFound, nil
}

func (r *ticketRepository) Find(ctx context.Context, id string) (*entity.Ticket, *errs.Error) {
	query := r.client.B().Get().Key(r.TicketDataKey(id)).Build()
	data, err := r.client.Do(ctx, query).AsBytes()
	if err != nil {
		return nil, entity.ErrTicketGetFailed.WithCause(err)
	}

	var ticket entity.Ticket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return nil, entity.ErrTicketUnmarshalFailed.WithCause(err)
	}

	return &ticket, nil
}

func (r *ticketRepository) Delete(ctx context.Context, target *entity.Ticket) *errs.Error {
	query := r.client.B().Del().Key(r.TicketDataKey(target.ID)).Build()
	if err := r.client.Do(ctx, query).Error(); err != nil {
		return entity.ErrTicketDeleteFailed.WithCause(err)
	}

	return nil
}
