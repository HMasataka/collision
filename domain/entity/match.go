package entity

import (
	"context"
	"time"
)

type Match struct {
	MatchId            string    `json:"match_id"`
	MatchProfile       string    `json:"match_profile"`
	MatchFunction      string    `json:"match_function"`
	Tickets            Tickets   `json:"tickets"`
	Backfill           *Backfill `json:"backfill"`
	Extensions         []byte    `json:"extensions"`
	AllocateGameserver bool      `json:"allocate_gameserver"`
}

type Backfill struct {
	ID              string
	SearchFields    *SearchFields
	Extensions      []byte
	PersistentField map[string]any
	CreateTime      time.Time
	Generation      int64
}

type DoubleRangeFilter_Exclude int32

const (
	DoubleRangeFilter_NONE DoubleRangeFilter_Exclude = 0
	DoubleRangeFilter_MIN  DoubleRangeFilter_Exclude = 1
	DoubleRangeFilter_MAX  DoubleRangeFilter_Exclude = 2
	DoubleRangeFilter_BOTH DoubleRangeFilter_Exclude = 3
)

type Pool struct {
	Name                string
	DoubleRangeFilters  []*DoubleRangeFilter
	StringEqualsFilters []*StringEqualsFilter
	TagPresentFilters   []*TagPresentFilter
	CreatedBefore       time.Time
	CreatedAfter        time.Time
}

type DoubleRangeFilter struct {
	DoubleArg string
	Max       float64
	Min       float64
	Exclude   DoubleRangeFilter_Exclude
}

type StringEqualsFilter struct {
	StringArg string
	Value     string
}

type TagPresentFilter struct {
	Tag string
}

type MatchProfile struct {
	Name       string
	Pools      []*Pool
	Extensions []byte
}

// MatchFunction performs matchmaking based on Ticket for each fetched Pool.
type MatchFunction interface {
	MakeMatches(ctx context.Context, profile *MatchProfile, poolTickets map[string][]*Ticket) ([]*Match, error)
}

type MatchFunctionFunc func(ctx context.Context, profile *MatchProfile, poolTickets map[string][]*Ticket) ([]*Match, error)

func (f MatchFunctionFunc) MakeMatches(ctx context.Context, profile *MatchProfile, poolTickets map[string][]*Ticket) ([]*Match, error) {
	return f(ctx, profile, poolTickets)
}

func (pf *Pool) In(ticket *Ticket) bool {
	if ticket == nil {
		return false
	}

	s := ticket.SearchFields

	if s == nil {
		s = &SearchFields{}
	}

	if !pf.CreatedAfter.IsZero() || !pf.CreatedBefore.IsZero() {
		ct := ticket.CreatedAt

		if !pf.CreatedAfter.IsZero() {
			if !ct.After(pf.CreatedAfter) {
				return false
			}
		}

		if !pf.CreatedBefore.IsZero() {
			if !ct.Before(pf.CreatedBefore) {
				return false
			}
		}
	}

	for _, f := range pf.DoubleRangeFilters {
		v, ok := s.DoubleArgs[f.DoubleArg]
		if !ok {
			return false
		}

		switch f.Exclude {
		case DoubleRangeFilter_NONE:
			// Not simplified so that NaN cases are handled correctly.
			if !(v >= f.Min && v <= f.Max) {
				return false
			}
		case DoubleRangeFilter_MIN:
			if !(v > f.Min && v <= f.Max) {
				return false
			}
		case DoubleRangeFilter_MAX:
			if !(v >= f.Min && v < f.Max) {
				return false
			}
		case DoubleRangeFilter_BOTH:
			if !(v > f.Min && v < f.Max) {
				return false
			}
		}

	}

	for _, f := range pf.StringEqualsFilters {
		v, ok := s.StringArgs[f.StringArg]
		if !ok {
			return false
		}
		if f.Value != v {
			return false
		}
	}

outer:
	for _, f := range pf.TagPresentFilters {
		for _, v := range s.Tags {
			if v == f.Tag {
				continue outer
			}
		}
		return false
	}

	return true
}
