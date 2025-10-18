//go:build wireinject
// +build wireinject

package di

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/infrastructure"
	"github.com/HMasataka/collision/infrastructure/persistence"
	"github.com/HMasataka/collision/usecase"
	"github.com/google/wire"
)

func InitializeUseCase(
	ctx context.Context,
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction,
	assigner entity.Assigner,
	evaluator entity.Evaluator,
) *usecase.UseCaseContainer {
	wire.Build(
		infrastructure.NewClient,
		infrastructure.NewLocker,
		persistence.NewTicketRepository,
		usecase.NewUseCaseOnce,
	)

	return nil
}
