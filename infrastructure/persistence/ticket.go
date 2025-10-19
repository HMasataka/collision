package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
)

const (
	defaultPendingReleaseTimeout = 1 * time.Minute
)

type ticketRepository struct {
	// NOTE 全体で共通の実態を持つ
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

func (r *ticketRepository) allTicketKey() string {
	return "alltickets"
}

func (r *ticketRepository) GetTickets(ctx context.Context, ticketIDs []string) ([]*entity.Ticket, []string, error) {
	keys := make([]string, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		keys[i] = r.TicketDataKey(ticketID)
	}

	m, err := rueidis.MGet(r.client, ctx, keys)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to mget tickets: %w", err)
	}

	tickets := make([]*entity.Ticket, 0, len(keys))
	var ticketIDsNotFound []string

	for key, resp := range m {
		if err := resp.Error(); err != nil {
			if rueidis.IsRedisNil(err) {
				ticketIDsNotFound = append(ticketIDsNotFound, r.ticketIDFromRedisKey(key))
				continue
			}
			return nil, nil, fmt.Errorf("failed to get tickets: %w", err)
		}

		data, err := resp.AsBytes()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode ticket as bytes: %w", err)
		}

		var ticket entity.Ticket

		if err := json.Unmarshal(data, &ticket); err != nil {
			return nil, nil, fmt.Errorf("failed to decode ticket: %w", err)
		}

		tickets = append(tickets, &ticket)
	}

	return tickets, ticketIDsNotFound, nil
}

func (r *ticketRepository) Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) error {
	data, err := json.Marshal(target)
	if err != nil {
		return err
	}

	queries := []rueidis.Completed{
		r.client.B().Set().
			Key(r.TicketDataKey(target.ID)).
			Value(rueidis.BinaryString(data)).
			Ex(ttl).
			Build(),
		r.client.B().Sadd().
			Key(r.allTicketKey()).
			Member(target.ID).
			Build(),
	}

	for _, resp := range r.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to create ticket: %w", err)
		}
	}

	return nil
}

func (r *ticketRepository) Find(ctx context.Context, id string) (*entity.Ticket, error) {
	query := r.client.B().Get().Key(r.TicketDataKey(id)).Build()
	data, err := r.client.Do(ctx, query).AsBytes()
	if err != nil {
		return nil, err
	}

	var ticket entity.Ticket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return nil, err
	}

	return &ticket, nil
}

func (r *ticketRepository) Delete(ctx context.Context, target *entity.Ticket) error {
	query := r.client.B().Del().Key(r.TicketDataKey(target.ID)).Build()
	if err := r.client.Do(ctx, query).Error(); err != nil {
		return err
	}

	return nil
}
