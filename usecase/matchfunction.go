package usecase

import (
	"context"
	"fmt"

	"github.com/HMasataka/collision/domain/entity"
)

func NewSimple1vs1MatchFunction() entity.MatchFunction {
	return entity.MatchFunctionFunc(func(ctx context.Context, profile *entity.MatchProfile, poolTickets map[string][]*entity.Ticket) ([]*entity.Match, error) {
		var matches []*entity.Match

		for _, tickets := range poolTickets {
			for len(tickets) >= 2 {
				match := newMatch(profile, tickets[:2])
				match.AllocateGameserver = true
				tickets = tickets[2:]
				matches = append(matches, match)
			}
		}

		return matches, nil
	})
}

func newMatch(profile *entity.MatchProfile, tickets entity.Tickets) *entity.Match {
	return &entity.Match{
		MatchId:       fmt.Sprintf("%s_%v", profile.Name, tickets.IDs()),
		MatchProfile:  profile.Name,
		MatchFunction: "Simple1vs1",
		Tickets:       tickets,
	}
}
