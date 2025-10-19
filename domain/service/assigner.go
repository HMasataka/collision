package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/errs"
	"github.com/redis/rueidis"
)

const (
	defaultAssignedDeleteTimeout = 1 * time.Minute
)

type AssignerService interface {
	GetAssignment(ctx context.Context, ticketID string) (*entity.Assignment, *errs.Error)
	AssignTickets(ctx context.Context, asgs []*entity.AssignmentGroup) ([]string, *errs.Error)
}

type assignerService struct {
	client             rueidis.Client
	ticketRepository   repository.TicketRepository
	ticketIDRepository repository.TicketIDRepository
	pendingRepository  repository.PendingTicketRepository
	ticketService      TicketService
}

func NewAssignerService(
	client rueidis.Client,
	repositoryContainer *repository.RepositoryContainer,
	ticketService TicketService,
) AssignerService {
	return &assignerService{
		client:             client,
		ticketRepository:   repositoryContainer.TicketRepository,
		ticketIDRepository: repositoryContainer.TicketIDRepository,
		pendingRepository:  repositoryContainer.PendingTicketRepository,
		ticketService:      ticketService,
	}
}

func (s *assignerService) assignmentData(ticketID string) string {
	return fmt.Sprintf("assign:%s", ticketID)
}

func (s *assignerService) GetAssignment(ctx context.Context, ticketID string) (*entity.Assignment, *errs.Error) {
	query := s.client.B().Get().Key(s.assignmentData(ticketID)).Build()

	resp := s.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, entity.ErrAssignmentNotFound
		}
		return nil, entity.ErrAssignmentGetFailed.WithCause(err)
	}

	data, err := resp.AsBytes()
	if err != nil {
		return nil, entity.ErrAssignmentGetFailed.WithCause(err)
	}

	var as entity.Assignment
	if err := json.Unmarshal(data, &as); err != nil {
		return nil, entity.ErrAssignmentDecodeFailed.WithCause(err)
	}

	return &as, nil
}

func (s *assignerService) AssignTickets(ctx context.Context, asgs []*entity.AssignmentGroup) ([]string, *errs.Error) {
	var assignedTicketIDs, notAssignedTicketIDs []string
	for _, asg := range asgs {
		if len(asg.TicketIds) == 0 {
			continue
		}
		// set assignment to a tickets
		redis := s.client

		if err := s.setAssignmentToTickets(ctx, redis, asg.TicketIds, asg.Assignment); err != nil {
			notAssignedTicketIDs = append(notAssignedTicketIDs, asg.TicketIds...)
			return notAssignedTicketIDs, err
		}
		assignedTicketIDs = append(assignedTicketIDs, asg.TicketIds...)
	}
	if len(assignedTicketIDs) > 0 {
		// de-index assigned tickets
		if err := s.ticketService.DeleteIndexTickets(ctx, assignedTicketIDs); err != nil {
			return notAssignedTicketIDs, entity.ErrTicketDeindexFailed.WithCause(err)
		}

		if err := s.setTicketsExpiration(ctx, assignedTicketIDs, defaultAssignedDeleteTimeout); err != nil {
			return notAssignedTicketIDs, err
		}

	}
	return notAssignedTicketIDs, nil
}

func (s *assignerService) setAssignmentToTickets(ctx context.Context, redis rueidis.Client, ticketIDs []string, assignment *entity.Assignment) *errs.Error {
	queries := make([]rueidis.Completed, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		data, err := json.Marshal(assignment)
		if err != nil {
			return entity.ErrAssignmentEncodeFailed.WithCause(err)
		}

		queries[i] = redis.B().Set().
			Key(s.assignmentData(ticketID)).
			Value(rueidis.BinaryString(data)).
			Ex(defaultAssignedDeleteTimeout).Build()
	}

	for _, resp := range redis.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return entity.ErrAssignmentSetFailed.WithCause(err)
		}
	}

	return nil
}

func (s *assignerService) setTicketsExpiration(ctx context.Context, ticketIDs []string, expiration time.Duration) *errs.Error {
	queries := make([]rueidis.Completed, len(ticketIDs))

	for i, ticketID := range ticketIDs {
		queries[i] = s.client.B().Expire().Key(s.ticketRepository.TicketDataKey(ticketID)).Seconds(int64(expiration.Seconds())).Build()
	}

	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return entity.ErrTicketExpirationFailed.WithCause(err)
		}
	}

	return nil
}
