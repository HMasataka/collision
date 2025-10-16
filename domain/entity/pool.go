package entity

import "time"

type Pool struct {
	Name                string
	DoubleRangeFilters  []*DoubleRangeFilter
	StringEqualsFilters []*StringEqualsFilter
	TagPresentFilters   []*TagPresentFilter
	CreatedBefore       time.Time
	CreatedAfter        time.Time
}

func (pf *Pool) In(ticket *Ticket) bool {
	if ticket == nil {
		return false
	}

	s := ticket.SearchFields
	if s == nil {
		s = &SearchFields{}
	}

	return pf.matchesCreatedTime(ticket.CreatedAt) &&
		pf.matchesDoubleRanges(s) &&
		pf.matchesStringEquals(s) &&
		pf.matchesTags(s)
}

func (pf *Pool) matchesCreatedTime(createdAt time.Time) bool {
	if pf.CreatedAfter.IsZero() && pf.CreatedBefore.IsZero() {
		return true
	}

	if !pf.CreatedAfter.IsZero() && !createdAt.After(pf.CreatedAfter) {
		return false
	}

	if !pf.CreatedBefore.IsZero() && !createdAt.Before(pf.CreatedBefore) {
		return false
	}

	return true
}

func (pf *Pool) matchesDoubleRanges(s *SearchFields) bool {
	for _, f := range pf.DoubleRangeFilters {
		v, ok := s.DoubleArgs[f.DoubleArg]
		if !ok {
			return false
		}

		if !f.isInRange(v) {
			return false
		}
	}

	return true
}

func (pf *Pool) matchesStringEquals(s *SearchFields) bool {
	for _, f := range pf.StringEqualsFilters {
		v, ok := s.StringArgs[f.StringArg]
		if !ok {
			return false
		}

		if f.Value != v {
			return false
		}
	}

	return true
}

func (pf *Pool) matchesTags(s *SearchFields) bool {
	for _, f := range pf.TagPresentFilters {
		if !f.isPresentIn(s.Tags) {
			return false
		}
	}

	return true
}
