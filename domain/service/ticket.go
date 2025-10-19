package service

import (
	"context"
	"fmt"

	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
)

type TicketService interface {
	GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error)
}

type ticketService struct {
	client             rueidis.Client
	locker             rueidislock.Locker
	ticketRepository   repository.TicketRepository
	ticketIDRepository repository.TicketIDRepository
	pendingRepository  repository.PendingTicketRepository
}

func NewTicketService(
	client rueidis.Client,
	locker rueidislock.Locker,
	repositoryContainer *repository.RepositoryContainer,
) TicketService {
	return &ticketService{
		client:             client,
		locker:             locker,
		ticketRepository:   repositoryContainer.TicketRepository,
		ticketIDRepository: repositoryContainer.TicketIDRepository,
		pendingRepository:  repositoryContainer.PendingTicketRepository,
	}
}

func (s *ticketService) fetchTicketsLock() string {
	return "fetchTicketsLock"
}

func (s *ticketService) GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	// 複数のワーカーが同時にFetchしないようにロックを取得する
	lockedCtx, unlock, err := s.locker.WithContext(ctx, s.fetchTicketsLock())
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
