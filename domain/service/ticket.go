package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HMasataka/collision/domain/driver"
	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
)

type TicketService interface {
	GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error)
	Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) error
	DeleteTicket(ctx context.Context, ticketID string) error
	DeleteIndexTickets(ctx context.Context, ticketIDs []string) error
}

type ticketService struct {
	client             rueidis.Client
	lockerDriver       driver.LockerDriver
	ticketRepository   repository.TicketRepository
	ticketIDRepository repository.TicketIDRepository
	pendingRepository  repository.PendingTicketRepository
}

func NewTicketService(
	client rueidis.Client,
	lockerDriver driver.LockerDriver,
	repositoryContainer *repository.RepositoryContainer,
) TicketService {
	return &ticketService{
		client:             client,
		lockerDriver:       lockerDriver,
		ticketRepository:   repositoryContainer.TicketRepository,
		ticketIDRepository: repositoryContainer.TicketIDRepository,
		pendingRepository:  repositoryContainer.PendingTicketRepository,
	}
}

func (s *ticketService) GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	// 複数のワーカーが同時にFetchしないようにロックを取得する
	lockedCtx, unlock, err := s.lockerDriver.FetchTicketLock(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	allTicketIDs, err := s.ticketIDRepository.GetAllTicketIDs(lockedCtx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all ticket IDs: %w", err)
	}
	if len(allTicketIDs) == 0 {
		return nil, nil
	}

	pendingTicketIDs, err := s.pendingRepository.GetPendingTicketIDs(lockedCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending ticket IDs: %w", err)
	}

	activeTicketIDs := difference(allTicketIDs, pendingTicketIDs)
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}

	if err := s.pendingRepository.InsertPendingTicket(lockedCtx, activeTicketIDs); err != nil {
		return nil, fmt.Errorf("failed to set tickets to pending: %w", err)
	}

	return activeTicketIDs, nil
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

func (s *ticketService) Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) error {
	data, err := json.Marshal(target)
	if err != nil {
		return err
	}

	queries := []rueidis.Completed{
		s.client.B().Set().
			Key(s.ticketRepository.TicketDataKey(target.ID)).
			Value(rueidis.BinaryString(data)).
			Ex(ttl).
			Build(),
		s.client.B().Sadd().
			Key(s.ticketIDRepository.TicketIDKey()).
			Member(target.ID).
			Build(),
	}

	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to create ticket: %w", err)
		}
	}

	return nil
}

func (s *ticketService) DeleteIndexTickets(ctx context.Context, ticketIDs []string) error {
	// Acquire locks to avoid race condition with GetActiveTicketIDs.
	//
	// Without locks, when the following order,
	// The assigned ticket is fetched again by the other backend, resulting in overlapping matches.
	//
	// 1. (GetActiveTicketIDs) getAllTicketIDs
	// 2. (deIndexTickets) ZREM and SREM from ticket index
	// 3. (GetActiveTicketIDs) getPendingTicketIDs
	lockedCtx, unlock, err := s.lockerDriver.FetchTicketLock(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	cmds := []rueidis.Completed{
		s.client.B().Zrem().Key(s.pendingRepository.PendingTicketKey()).Member(ticketIDs...).Build(),
		s.client.B().Srem().Key(s.ticketIDRepository.TicketIDKey()).Member(ticketIDs...).Build(),
	}

	for _, resp := range s.client.DoMulti(lockedCtx, cmds...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to deindex tickets: %w", err)
		}
	}

	return nil
}

func (s *ticketService) DeleteTicket(ctx context.Context, ticketID string) error {
	lockedCtx, unlock, err := s.lockerDriver.FetchTicketLock(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	queries := []rueidis.Completed{
		s.client.B().Del().Key(s.ticketRepository.TicketDataKey(ticketID)).Build(),
		s.client.B().Srem().Key(s.ticketIDRepository.TicketIDKey()).Member(ticketID).Build(),
		s.client.B().Zrem().Key(s.pendingRepository.PendingTicketKey()).Member(ticketID).Build(),
	}
	for _, resp := range s.client.DoMulti(lockedCtx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to delete ticket: %w", err)
		}
	}

	return nil
}
