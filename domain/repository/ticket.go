package repository

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
)

type TicketRepository interface {
	TicketDataKey(ticketID string) string

	GetTickets(ctx context.Context, ticketIDs []string) ([]*entity.Ticket, []string, error)
	Find(ctx context.Context, id string) (*entity.Ticket, error)
	Delete(ctx context.Context, target *entity.Ticket) error
}
