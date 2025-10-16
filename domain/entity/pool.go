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
		case DoubleRangeFilterNone:
			// Not simplified so that NaN cases are handled correctly.
			if !(f.Min <= v && v <= f.Max) {
				return false
			}
		case DoubleRangeFilterMin:
			if !(f.Min < v && v <= f.Max) {
				return false
			}
		case DoubleRangeFilterMax:
			if !(f.Min <= v && v < f.Max) {
				return false
			}
		case DoubleRangeFilterBoth:
			if !(f.Min < v && v < f.Max) {
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
