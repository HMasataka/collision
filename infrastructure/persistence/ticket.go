package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
)

const (
	defaultPendingReleaseTimeout = 1 * time.Minute
)

type ticketRepository struct {
	// NOTE 全体で共通の実態を持つ
	locker rueidislock.Locker
	client rueidis.Client
}

func NewTicketRepository(
	locker rueidislock.Locker,
	client rueidis.Client,
) repository.TicketRepository {
	return &ticketRepository{
		locker: locker,
		client: client,
	}
}

func (r *ticketRepository) ticketDataKey(ticketID string) string {
	return ticketID
}

func (r *ticketRepository) ticketIDFromRedisKey(key string) string {
	return key
}

func (r *ticketRepository) allTicketKey() string {
	return "alltickets"
}

func (r *ticketRepository) pendingTicketKey() string {
	return "proposed_ticket_ids"
}

func (r *ticketRepository) GetAllTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	query := r.client.B().Srandmember().Key(r.allTicketKey()).Count(limit).Build()

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

func (r *ticketRepository) GetPendingTicketIDs(ctx context.Context) ([]string, error) {
	rangeMin := strconv.FormatInt(time.Now().Add(-defaultPendingReleaseTimeout).Unix(), 10)
	rangeMax := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)

	query := r.client.B().Zrangebyscore().Key(r.pendingTicketKey()).Min(rangeMin).Max(rangeMax).Build()

	resp := r.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending ticket index: %w", err)
	}

	pendingTicketIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to decode pending ticket index as str slice: %w", err)
	}

	return pendingTicketIDs, nil
}

func (r *ticketRepository) InsertPendingTicket(ctx context.Context, ticketIDs []string) error {
	score := float64(time.Now().Unix())

	query := r.client.B().Zadd().Key(r.pendingTicketKey()).ScoreMember()
	for _, ticketID := range ticketIDs {
		query = query.ScoreMember(score, ticketID)
	}

	resp := r.client.Do(ctx, query.Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("failed to set tickets to pending state: %w", err)
	}

	return nil
}

func (r *ticketRepository) GetTickets(ctx context.Context, ticketIDs []string) ([]*entity.Ticket, []string, error) {
	keys := make([]string, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		keys[i] = r.ticketDataKey(ticketID)
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
			Key(r.ticketDataKey(target.ID)).
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
	query := r.client.B().Get().Key(r.ticketDataKey(id)).Build()
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
	query := r.client.B().Del().Key(r.ticketDataKey(target.ID)).Build()
	if err := r.client.Do(ctx, query).Error(); err != nil {
		return err
	}

	return nil
}
