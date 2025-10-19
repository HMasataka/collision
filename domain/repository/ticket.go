package repository

import (
	"context"
	"time"

	"github.com/HMasataka/collision/domain/entity"
)

type TicketRepository interface {
	GetTickets(ctx context.Context, ticketIDs []string) ([]*entity.Ticket, []string, error)
	Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) error
	Find(ctx context.Context, id string) (*entity.Ticket, error)

	DeleteTicket(ctx context.Context, ticketID string) error

	GetAssignment(ctx context.Context, ticketID string) (*entity.Assignment, error)
	AssignTickets(ctx context.Context, asgs []*entity.AssignmentGroup) ([]string, error)
	DeleteIndexTickets(ctx context.Context, ticketIDs []string) error
}
