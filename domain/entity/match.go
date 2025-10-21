package entity

import (
	"context"
	"time"

	"github.com/samber/lo"
)

type Match struct {
	MatchID            string    `json:"match_id"`
	MatchProfile       string    `json:"match_profile"`
	MatchFunction      string    `json:"match_function"`
	Tickets            Tickets   `json:"tickets"`
	Backfill           *Backfill `json:"backfill"`
	Extensions         []byte    `json:"extensions"`
	AllocateGameserver bool      `json:"allocate_gameserver"`
}

type Matches []*Match

func (m Matches) TicketIDs() []string {
	return lo.FlatMap(m, func(match *Match, _ int) []string {
		return match.Tickets.IDs()
	})
}

type Backfill struct {
	ID              string
	SearchFields    *SearchFields
	Extensions      []byte
	PersistentField map[string]any
	CreateTime      time.Time
	Generation      int64
}

type MatchProfile struct {
	Name       string
	Pools      []*Pool
	Extensions []byte
}

// MatchFunction performs matchmaking based on Ticket for each fetched Pool.
type MatchFunction interface {
	MakeMatches(ctx context.Context, profile *MatchProfile, poolTickets map[string][]*Ticket) (Matches, error)
}

type MatchFunctionFunc func(ctx context.Context, profile *MatchProfile, poolTickets map[string][]*Ticket) (Matches, error)

func (f MatchFunctionFunc) MakeMatches(ctx context.Context, profile *MatchProfile, poolTickets map[string][]*Ticket) (Matches, error) {
	return f(ctx, profile, poolTickets)
}
