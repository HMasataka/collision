package entity

import (
	"context"
)

type Assignment struct {
	Connection string `json:"connection"`
	Extensions []byte `json:"extensions"`
}

type AssignmentGroup struct {
	TicketIds  []string
	Assignment *Assignment
}

// Assigner assigns a GameServer info to the established matches.
type Assigner interface {
	Assign(ctx context.Context, matches []*Match) ([]*AssignmentGroup, error)
}

type AssignerFunc func(ctx context.Context, matches []*Match) ([]*AssignmentGroup, error)

func (f AssignerFunc) Assign(ctx context.Context, matches []*Match) ([]*AssignmentGroup, error) {
	return f(ctx, matches)
}
