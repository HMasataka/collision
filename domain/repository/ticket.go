package repository

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/errs"
)

type TicketRepository interface {
	TicketDataKey(ticketID string) string

	GetTickets(ctx context.Context, ticketIDs []string) ([]*entity.Ticket, []string, *errs.Error)
	Find(ctx context.Context, id string) (*entity.Ticket, *errs.Error)
	Delete(ctx context.Context, target *entity.Ticket) *errs.Error
}
