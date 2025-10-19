package repository

import (
	"context"

	"github.com/HMasataka/errs"
)

type TicketIDRepository interface {
	TicketIDKey() string

	GetAllTicketIDs(ctx context.Context, limit int64) ([]string, *errs.Error)
}
