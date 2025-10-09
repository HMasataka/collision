package repository

import (
	"context"
	"time"

	"github.com/HMasataka/collision/domain/entity"
)

type TicketRepository interface {
	Insert(ctx context.Context, target *entity.Ticket, ttl time.Duration) error
	Find(ctx context.Context, id string) (*entity.Ticket, error)
	Delete(ctx context.Context, target *entity.Ticket) error
}
