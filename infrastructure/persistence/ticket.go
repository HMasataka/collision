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
	defaultAssignedDeleteTimeout = 1 * time.Minute
)

type ticketRepository struct {
	// NOTE 全体で共通の実態を持つ
	locker rueidislock.Locker
	client rueidis.Client
}

func NewTicketRepository(
	client rueidis.Client,
	locker rueidislock.Locker,
) repository.TicketRepository {
	return &ticketRepository{
		client: client,
		locker: locker,
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
	return "proposedTicketIDs"
}

func (r *ticketRepository) fetchTicketsLock() string {
	return "fetchTicketsLock"
}

func (r *ticketRepository) assignmentData(ticketID string) string {
	return fmt.Sprintf("assign:%s", ticketID)
}

// TODO serviceに移動する
func (r *ticketRepository) GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	// 複数のワーカーが同時にFetchしないようにロックを取得する
	lockedCtx, unlock, err := r.locker.WithContext(ctx, r.fetchTicketsLock())
	if err != nil {
		return nil, fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	allTicketIDs, err := r.GetAllTicketIDs(lockedCtx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all ticket IDs: %w", err)
	}
	if len(allTicketIDs) == 0 {
		return nil, nil
	}

	pendingTicketIDs, err := r.GetPendingTicketIDs(lockedCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending ticket IDs: %w", err)
	}

	activeTicketIDs := difference(allTicketIDs, pendingTicketIDs)
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}

	if err := r.InsertPendingTicket(lockedCtx, activeTicketIDs); err != nil {
		return nil, fmt.Errorf("failed to set tickets to pending: %w", err)
	}

	return activeTicketIDs, nil
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

func (r *ticketRepository) ReleaseTickets(ctx context.Context, ticketIDs []string) error {
	lockedCtx, unlock, err := r.locker.WithContext(ctx, r.fetchTicketsLock())
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	query := r.client.B().Zrem().Key(r.pendingTicketKey()).Member(ticketIDs...).Build()

	resp := r.client.Do(lockedCtx, query)
	if err := resp.Error(); err != nil {
		return fmt.Errorf("failed to release tickets: %w", err)
	}

	return nil
}

func (r *ticketRepository) GetAssignment(ctx context.Context, ticketID string) (*entity.Assignment, error) {
	query := r.client.B().Get().Key(r.assignmentData(ticketID)).Build()

	resp := r.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, entity.ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("failed to get assignemnt: %w", err)
	}

	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment as bytes: %w", err)
	}

	var as entity.Assignment
	if err := json.Unmarshal(data, &as); err != nil {
		return nil, fmt.Errorf("failed to decode assignment: %w", err)
	}

	return &as, nil
}

func (r *ticketRepository) AssignTickets(ctx context.Context, asgs []*entity.AssignmentGroup) ([]string, error) {
	var assignedTicketIDs, notAssignedTicketIDs []string
	for _, asg := range asgs {
		if len(asg.TicketIds) == 0 {
			continue
		}
		// set assignment to a tickets
		redis := r.client

		if err := r.setAssignmentToTickets(ctx, redis, asg.TicketIds, asg.Assignment); err != nil {
			notAssignedTicketIDs = append(notAssignedTicketIDs, asg.TicketIds...)
			return notAssignedTicketIDs, err
		}
		assignedTicketIDs = append(assignedTicketIDs, asg.TicketIds...)
	}
	if len(assignedTicketIDs) > 0 {
		// de-index assigned tickets
		if err := r.DeleteIndexTickets(ctx, assignedTicketIDs); err != nil {
			return notAssignedTicketIDs, fmt.Errorf("failed to deindex assigned tickets: %w", err)
		}

		if err := r.setTicketsExpiration(ctx, assignedTicketIDs, defaultAssignedDeleteTimeout); err != nil {
			return notAssignedTicketIDs, err
		}

	}
	return notAssignedTicketIDs, nil
}

func (r *ticketRepository) setAssignmentToTickets(ctx context.Context, redis rueidis.Client, ticketIDs []string, assignment *entity.Assignment) error {
	queries := make([]rueidis.Completed, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		data, err := json.Marshal(assignment)
		if err != nil {
			return fmt.Errorf("failed to encode assignemnt: %w", err)
		}

		queries[i] = redis.B().Set().
			Key(r.assignmentData(ticketID)).
			Value(rueidis.BinaryString(data)).
			Ex(defaultAssignedDeleteTimeout).Build()
	}

	for _, resp := range redis.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to set assignemnt data to redis: %w", err)
		}
	}

	return nil
}

func (r *ticketRepository) setTicketsExpiration(ctx context.Context, ticketIDs []string, expiration time.Duration) error {
	queries := make([]rueidis.Completed, len(ticketIDs))

	for i, ticketID := range ticketIDs {
		queries[i] = r.client.B().Expire().Key(r.ticketDataKey(ticketID)).Seconds(int64(expiration.Seconds())).Build()
	}

	for _, resp := range r.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to set expiration to tickets: %w", err)
		}
	}

	return nil
}

func (r *ticketRepository) DeleteIndexTickets(ctx context.Context, ticketIDs []string) error {
	// Acquire locks to avoid race condition with GetActiveTicketIDs.
	//
	// Without locks, when the following order,
	// The assigned ticket is fetched again by the other backend, resulting in overlapping matches.
	//
	// 1. (GetActiveTicketIDs) getAllTicketIDs
	// 2. (deIndexTickets) ZREM and SREM from ticket index
	// 3. (GetActiveTicketIDs) getPendingTicketIDs
	lockedCtx, unlock, err := r.locker.WithContext(ctx, r.fetchTicketsLock())
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	cmds := []rueidis.Completed{
		r.client.B().Zrem().Key(r.pendingTicketKey()).Member(ticketIDs...).Build(),
		r.client.B().Srem().Key(r.allTicketKey()).Member(ticketIDs...).Build(),
	}

	for _, resp := range r.client.DoMulti(lockedCtx, cmds...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to deindex tickets: %w", err)
		}
	}

	return nil
}

// difference returns the elements in `a` that aren't in `b`.
// https://stackoverflow.com/a/45428032
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func (r *ticketRepository) DeleteTicket(ctx context.Context, ticketID string) error {
	lockedCtx, unlock, err := r.locker.WithContext(ctx, r.fetchTicketsLock())
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	queries := []rueidis.Completed{
		r.client.B().Del().Key(r.ticketDataKey(ticketID)).Build(),
		r.client.B().Srem().Key(r.allTicketKey()).Member(ticketID).Build(),
		r.client.B().Zrem().Key(r.pendingTicketKey()).Member(ticketID).Build(),
	}
	for _, resp := range r.client.DoMulti(lockedCtx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to delete ticket: %w", err)
		}
	}

	return nil
}
