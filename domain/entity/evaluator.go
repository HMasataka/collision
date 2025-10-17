package entity

import (
	"context"
)

type Evaluator interface {
	Evaluate(ctx context.Context, matches []*Match) ([]string, error)
}

type EvaluatorFunc func(ctx context.Context, matches []*Match) ([]string, error)

func (f EvaluatorFunc) Evaluate(ctx context.Context, matches []*Match) ([]string, error) {
	return f(ctx, matches)
}
