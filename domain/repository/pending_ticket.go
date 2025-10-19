package repository

import (
	"context"

	"github.com/HMasataka/errs"
)

type PendingTicketRepository interface {
	PendingTicketKey() string

	GetPendingTicketIDs(ctx context.Context) ([]string, *errs.Error)
	InsertPendingTicket(ctx context.Context, ticketIDs []string) *errs.Error
	ReleaseTickets(ctx context.Context, ticketIDs []string) *errs.Error
}
