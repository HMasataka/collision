package repository

import (
	"context"
)

type TicketIDRepository interface {
	TicketIDKey() string

	GetAllTicketIDs(ctx context.Context, limit int64) ([]string, error)
}
