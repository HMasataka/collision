package repository

import (
	"context"
	"time"

	"github.com/HMasataka/collision/domain/entity"
)

type TicketRepository interface {
	GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error)
	GetAllTicketIDs(ctx context.Context, limit int64) ([]string, error)
	GetPendingTicketIDs(ctx context.Context) ([]string, error)
	InsertPendingTicket(ctx context.Context, ticketIDs []string) error
	GetTickets(ctx context.Context, ticketIDs []string) ([]*entity.Ticket, []string, error)
	Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) error
	Find(ctx context.Context, id string) (*entity.Ticket, error)
	Delete(ctx context.Context, target *entity.Ticket) error

	GetAssignment(ctx context.Context, ticketID string) (*entity.Assignment, error)
	AssignTickets(ctx context.Context, asgs []*entity.AssignmentGroup) ([]string, error)
	ReleaseTickets(ctx context.Context, ticketIDs []string) error
	DeleteIndexTickets(ctx context.Context, ticketIDs []string) error
}
