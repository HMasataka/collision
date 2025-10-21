package entity

import (
	"time"

	"github.com/samber/lo"
)

type Ticket struct {
	ID              string         `json:"id"`
	Assignment      *Assignment    `json:"assignment"`
	SearchFields    *SearchFields  `json:"search_fields"`
	Extensions      []byte         `json:"extensions"`
	PersistentField map[string]any `json:"persistent_field"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Tickets []*Ticket

func (t Tickets) IDs() []string {
	ids := make([]string, 0, len(t))

	for _, ticket := range t {
		if ticket == nil {
			continue
		}

		ids = append(ids, ticket.ID)
	}

	return ids
}

// SplitByIDs splits tickets into matched and unmatched groups based on the provided IDs
func (t Tickets) SplitByIDs(ids []string) (matched Tickets, unmatched Tickets) {
	idSet := lo.SliceToMap(ids, func(id string) (string, struct{}) {
		return id, struct{}{}
	})

	matched = lo.Filter(t, func(ticket *Ticket, _ int) bool {
		if ticket == nil {
			return false
		}
		_, exists := idSet[ticket.ID]
		return exists
	})

	unmatched = lo.Filter(t, func(ticket *Ticket, _ int) bool {
		if ticket == nil {
			return false
		}
		_, exists := idSet[ticket.ID]
		return !exists
	})

	return matched, unmatched
}

type SearchFields struct {
	DoubleArgs map[string]float64 `json:"double_args"`
	StringArgs map[string]string  `json:"string_args"`
	Tags       []string           `json:"tags"`
}
