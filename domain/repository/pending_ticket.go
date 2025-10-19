package repository

import (
	"context"
)

type PendingTicketRepository interface {
	PendingTicketKey() string

	GetPendingTicketIDs(ctx context.Context) ([]string, error)
	InsertPendingTicket(ctx context.Context, ticketIDs []string) error
	ReleaseTickets(ctx context.Context, ticketIDs []string) error
}
