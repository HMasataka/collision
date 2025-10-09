package usecase

import (
	"context"
	"time"

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
	return &ticketUsecase{
		ticketRepository: ticketRepository,
	}
}

func (u *ticketUsecase) CreateTicket(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) (*entity.Ticket, error) {
	id := xid.New().String()

	ticket := &entity.Ticket{
		ID:           id,
		SearchFields: searchFields,
	}

	if err := u.ticketRepository.Insert(ctx, ticket, 10*time.Minute); err != nil {
		return nil, err
	}

	return ticket, nil
}
