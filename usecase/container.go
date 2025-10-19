package usecase

import (
	"sync"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/collision/domain/service"
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
	ticketService service.TicketService,
	assignerService service.AssignerService,
) *UseCaseContainer {
	once.Do(func() {
		container = newContainer(matchFunctions, assigner, evaluator, repositoryContainer, ticketService, assignerService)
	})

	return container
}

func newContainer(
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction,
	assigner entity.Assigner,
	evaluator entity.Evaluator,
	repositoryContainer *repository.RepositoryContainer,
	ticketService service.TicketService,
	assignerService service.AssignerService,
) *UseCaseContainer {
	return &UseCaseContainer{
		MatchUsecase:  NewMatchUsecase(matchFunctions, assigner, evaluator, repositoryContainer, ticketService, assignerService),
		TicketUsecase: NewTicketUsecase(repositoryContainer, ticketService),
		AssignUsecase: NewAssignUsecase(assignerService),
	}
}
