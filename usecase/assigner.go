package usecase

import (
	"context"
	"log"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/bojand/hri"
)

func NewRandomAssigner() entity.Assigner {
	return entity.AssignerFunc(func(ctx context.Context, matches entity.Matches) ([]*entity.AssignmentGroup, error) {
		var asgs []*entity.AssignmentGroup

		for _, match := range matches {
			ticketIDs := match.Tickets.IDs()
			conn := hri.Random()
			log.Printf("assign '%s' to tickets: %v", conn, ticketIDs)

			asgs = append(asgs, &entity.AssignmentGroup{
				TicketIds:  ticketIDs,
				Assignment: &entity.Assignment{Connection: conn},
			})
		}

		return asgs, nil
	})
}
