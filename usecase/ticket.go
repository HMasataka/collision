package usecase

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/rs/xid"
)

type TicketUsecase interface {
	CreateTicket(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) (*entity.Ticket, error)
}

type ticketUsecase struct {
	ticketRepository repository.TicketRepository
}

func NewTicketUsecase(
	ticketRepository repository.TicketRepository,
) TicketUsecase {
	return &ticketUsecase{}
}

func (u *ticketUsecase) CreateTicket(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) (*entity.Ticket, error) {
	id := xid.New().String()

	ticket := &entity.Ticket{
		ID:           id,
		SearchFields: searchFields,
	}

	if err := u.ticketRepository.WithLock(ctx, id, func(ctx context.Context) error {
		return u.ticketRepository.Insert(ctx, ticket)
	}); err != nil {
		return nil, err
	}

	return ticket, nil
}
