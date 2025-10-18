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
	ticketRepository repository.TicketRepository,
) *UseCaseContainer {
	once.Do(func() {
		container = newContainer(matchFunctions, assigner, evaluator, ticketRepository)
	})

	return container
}

func newContainer(
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction,
	assigner entity.Assigner,
	evaluator entity.Evaluator,
	ticketRepository repository.TicketRepository,
) *UseCaseContainer {
	return &UseCaseContainer{
		MatchUsecase:  NewMatchUsecase(matchFunctions, assigner, evaluator, ticketRepository),
		TicketUsecase: NewTicketUsecase(ticketRepository),
		AssignUsecase: NewAssignUsecase(ticketRepository),
	}
}
