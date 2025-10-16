package entity

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
