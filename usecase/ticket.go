package usecase

import (
	"context"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/service"
	"github.com/HMasataka/errs"
	"github.com/rs/xid"
)

type TicketUsecase interface {
	CreateTicket(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) (*entity.Ticket, *errs.Error)
	DeleteTicket(ctx context.Context, ticketID string) *errs.Error
}

type ticketUsecase struct {
	ticketService service.TicketService
}

func NewTicketUsecase(
	ticketService service.TicketService,
) TicketUsecase {
	return &ticketUsecase{
		ticketService: ticketService,
	}
}

func (u *ticketUsecase) CreateTicket(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) (*entity.Ticket, *errs.Error) {
	id := xid.New().String()

	ticket := &entity.Ticket{
		ID:           id,
		SearchFields: searchFields,
	}

	if err := u.ticketService.Insert(ctx, ticket, 10*time.Minute); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (u *ticketUsecase) DeleteTicket(ctx context.Context, ticketID string) *errs.Error {
	return u.ticketService.DeleteTicket(ctx, ticketID)
}
