package persistence

import (
	"context"
	"fmt"

	"github.com/HMasataka/collision/domain/repository"
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

func (r *ticketIDRepository) GetAllTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	query := r.client.B().Srandmember().Key(r.TicketIDKey()).Count(limit).Build()

	resp := r.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get all tickets index: %w", err)
	}

	allTicketIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to decode all tickets index as str slice: %w", err)
	}

	return allTicketIDs, nil
}
