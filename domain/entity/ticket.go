package entity

import (
	"time"
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

type SearchFields struct {
	DoubleArgs map[string]float64 `json:"double_args"`
	StringArgs map[string]string  `json:"string_args"`
	Tags       []string           `json:"tags"`
}
