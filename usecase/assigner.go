package usecase

import (
	"context"
	"log"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/bojand/hri"
)

func NewRandomAssigner() entity.Assigner {
	return entity.AssignerFunc(func(ctx context.Context, matches []*entity.Match) ([]*entity.AssignmentGroup, error) {
		var asgs []*entity.AssignmentGroup

		for _, match := range matches {
			tids := ticketIDs(match)
			conn := hri.Random()
			log.Printf("assign '%s' to tickets: %v", conn, tids)
			asgs = append(asgs, &entity.AssignmentGroup{
				TicketIds:  tids,
				Assignment: &entity.Assignment{Connection: conn},
			})
		}

		return asgs, nil
	})
}

func ticketIDs(match *entity.Match) []string {
	var ids []string

	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.ID)
	}

	return ids
}
