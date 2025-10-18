package usecase

import (
	"sync"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
)

type UseCaseContainer struct {
	MatchUsecase  MatchUsecase
	TicketUsecase TicketUsecase
	AssignUsecase AssignUsecase
}

var (
	container *UseCaseContainer
	once      sync.Once
)

func NewUseCaseOnce(
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction,
	assigner entity.Assigner,
	evaluator entity.Evaluator,
	repositoryContainer *repository.RepositoryContainer,
) *UseCaseContainer {
	once.Do(func() {
		container = newContainer(matchFunctions, assigner, evaluator, repositoryContainer)
	})

	return container
}

func newContainer(
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction,
	assigner entity.Assigner,
	evaluator entity.Evaluator,
	repositoryContainer *repository.RepositoryContainer,
) *UseCaseContainer {
	return &UseCaseContainer{
		MatchUsecase:  NewMatchUsecase(matchFunctions, assigner, evaluator, repositoryContainer),
		TicketUsecase: NewTicketUsecase(repositoryContainer),
		AssignUsecase: NewAssignUsecase(repositoryContainer),
	}
}
