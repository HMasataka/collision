package repository

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
)

type TicketRepository interface {
	WithLock(ctx context.Context, id string, fn func(ctx context.Context) error) error
	Insert(ctx context.Context, target *entity.Ticket) error
	Find(ctx context.Context, id string) (*entity.Ticket, error)
	Delete(ctx context.Context, target *entity.Ticket) error
}
