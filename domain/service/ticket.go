package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/HMasataka/collision/domain/driver"
	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/errs"
	"github.com/redis/rueidis"
	"github.com/samber/lo"
)

type TicketService interface {
	GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, *errs.Error)
	Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) *errs.Error
	DeleteTicket(ctx context.Context, ticketID string) *errs.Error
	DeleteIndexTickets(ctx context.Context, ticketIDs []string) *errs.Error
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

func (s *ticketService) GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, *errs.Error) {
	// 複数のワーカーが同時にFetchしないようにロックを取得する
	lockedCtx, unlock, err := s.lockerDriver.FetchTicketLock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	allTicketIDs, err := s.ticketIDRepository.GetAllTicketIDs(lockedCtx, limit)
	if err != nil {
		return nil, entity.ErrIndexGetFailed.WithCause(err)
	}
	if len(allTicketIDs) == 0 {
		return nil, nil
	}

	pendingTicketIDs, err := s.pendingRepository.GetPendingTicketIDs(lockedCtx)
	if err != nil {
		return nil, entity.ErrPendingTicketGetFailed.WithCause(err)
	}

	activeTicketIDs, _ := lo.Difference(allTicketIDs, pendingTicketIDs)
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}

	if err := s.pendingRepository.InsertPendingTicket(lockedCtx, activeTicketIDs); err != nil {
		return nil, entity.ErrPendingTicketSetFailed.WithCause(err)
	}

	return activeTicketIDs, nil
}

func (s *ticketService) Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) *errs.Error {
	data, err := json.Marshal(target)
	if err != nil {
		return entity.ErrTicketMarshalFailed.WithCause(err)
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
			return entity.ErrTicketCreateFailed.WithCause(err)
		}
	}

	return nil
}

func (s *ticketService) DeleteIndexTickets(ctx context.Context, ticketIDs []string) *errs.Error {
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
		return err
	}
	defer unlock()

	cmds := []rueidis.Completed{
		s.client.B().Zrem().Key(s.pendingRepository.PendingTicketKey()).Member(ticketIDs...).Build(),
		s.client.B().Srem().Key(s.ticketIDRepository.TicketIDKey()).Member(ticketIDs...).Build(),
	}

	for _, resp := range s.client.DoMulti(lockedCtx, cmds...) {
		if err := resp.Error(); err != nil {
			return entity.ErrTicketDeindexFailed.WithCause(err)
		}
	}

	return nil
}

func (s *ticketService) DeleteTicket(ctx context.Context, ticketID string) *errs.Error {
	lockedCtx, unlock, err := s.lockerDriver.FetchTicketLock(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	queries := []rueidis.Completed{
		s.client.B().Del().Key(s.ticketRepository.TicketDataKey(ticketID)).Build(),
		s.client.B().Srem().Key(s.ticketIDRepository.TicketIDKey()).Member(ticketID).Build(),
		s.client.B().Zrem().Key(s.pendingRepository.PendingTicketKey()).Member(ticketID).Build(),
	}
	for _, resp := range s.client.DoMulti(lockedCtx, queries...) {
		if err := resp.Error(); err != nil {
			return entity.ErrTicketDeleteFailed.WithCause(err)
		}
	}

	return nil
}
