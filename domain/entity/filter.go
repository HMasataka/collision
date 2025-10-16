package entity

import "slices"

type DoubleRangeFilterExclude int32

const (
	DoubleRangeFilterNone DoubleRangeFilterExclude = 0
	DoubleRangeFilterMin  DoubleRangeFilterExclude = 1
	DoubleRangeFilterMax  DoubleRangeFilterExclude = 2
	DoubleRangeFilterBoth DoubleRangeFilterExclude = 3
)

type DoubleRangeFilter struct {
	DoubleArg string
	Max       float64
	Min       float64
	Exclude   DoubleRangeFilterExclude
}

type StringEqualsFilter struct {
	StringArg string
	Value     string
}

type TagPresentFilter struct {
	Tag string
}

func (f *DoubleRangeFilter) isInRange(v float64) bool {
	switch f.Exclude {
	case DoubleRangeFilterNone:
		return f.Min <= v && v <= f.Max
	case DoubleRangeFilterMin:
		return f.Min < v && v <= f.Max
	case DoubleRangeFilterMax:
		return f.Min <= v && v < f.Max
	case DoubleRangeFilterBoth:
		return f.Min < v && v < f.Max
	default:
		return false
	}
}

func (f *TagPresentFilter) isPresentIn(tags []string) bool {
	return slices.Contains(tags, f.Tag)
}
