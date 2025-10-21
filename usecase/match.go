package usecase

import (
	"context"
	"sync"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/collision/domain/service"
	"github.com/HMasataka/errs"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

type MatchUsecase interface {
	Exec(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) *errs.Error
}

type matchUsecase struct {
	mutex          sync.RWMutex
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction

	assigner  entity.Assigner
	evaluator entity.Evaluator

	ticketRepository        repository.TicketRepository
	pendingTicketRepository repository.PendingTicketRepository
	ticketService           service.TicketService
	assignerService         service.AssignerService
}

func NewMatchUsecase(
	matchFunctions map[*entity.MatchProfile]entity.MatchFunction,
	assigner entity.Assigner,
	evaluator entity.Evaluator,
	repositoryContainer *repository.RepositoryContainer,
	ticketService service.TicketService,
	assignerService service.AssignerService,
) MatchUsecase {
	return &matchUsecase{
		mutex:                   sync.RWMutex{},
		matchFunctions:          matchFunctions,
		assigner:                assigner,
		evaluator:               evaluator,
		ticketRepository:        repositoryContainer.TicketRepository,
		pendingTicketRepository: repositoryContainer.PendingTicketRepository,
		ticketService:           ticketService,
		assignerService:         assignerService,
	}
}

func (u *matchUsecase) Exec(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) *errs.Error {
	activeTickets, err := u.fetchActiveTickets(ctx, 10000)
	if err != nil {
		return err
	}

	if len(activeTickets) == 0 {
		return nil
	}

	matches, err := u.makeMatches(ctx, activeTickets)
	if err != nil {
		return err
	}

	matches, err = u.evaluateMatches(ctx, matches)
	if err != nil {
		return err
	}

	unmatchedTicketIDs := filterUnmatchedTicketIDs(activeTickets, matches.TicketIDs())
	if len(unmatchedTicketIDs) > 0 {
		if err := u.pendingTicketRepository.ReleaseTickets(ctx, unmatchedTicketIDs); err != nil {
			return entity.ErrPendingTicketReleaseFailed.WithCause(err)
		}
	}

	if len(matches) > 0 {
		if err := u.assign(ctx, matches); err != nil {
			return err
		}
	}

	return nil

}

func (u *matchUsecase) fetchActiveTickets(ctx context.Context, limit int64) (entity.Tickets, *errs.Error) {
	activeTicketIDs, err := u.ticketService.GetActiveTicketIDs(ctx, limit)
	if err != nil {
		return nil, entity.ErrIndexGetFailed.WithCause(err)
	}
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}

	tickets, deletedTicketIDs, err := u.ticketRepository.GetTickets(ctx, activeTicketIDs)
	if err != nil {
		return nil, entity.ErrTicketGetFailed.WithCause(err)
	}

	if len(deletedTicketIDs) > 0 {
		if err := u.ticketService.DeleteIndexTickets(ctx, deletedTicketIDs); err != nil {
			return nil, entity.ErrIndexDeleteFailed.WithCause(err)
		}
	}

	return tickets, nil
}

func (u *matchUsecase) makeMatches(ctx context.Context, activeTickets entity.Tickets) (entity.Matches, *errs.Error) {
	u.mutex.RLock()
	mmfs := u.matchFunctions
	u.mutex.RUnlock()

	resCh := make(chan entity.Matches, len(mmfs))
	eg, ctx := errgroup.WithContext(ctx)

	for profile, mmf := range mmfs {
		eg.Go(func() error {
			poolTickets, err := filterTickets(profile, activeTickets)
			if err != nil {
				return err
			}

			matches, err := mmf.MakeMatches(ctx, profile, poolTickets)
			if err != nil {
				return err
			}

			resCh <- matches

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, entity.ErrMatchExecutionFailed.WithCause(err)
	}

	close(resCh)

	var totalMatches entity.Matches
	for matches := range resCh {
		totalMatches = append(totalMatches, matches...)
	}

	return totalMatches, nil
}

func filterTickets(profile *entity.MatchProfile, tickets entity.Tickets) (map[string]entity.Tickets, error) {
	poolTickets := map[string]entity.Tickets{}

	for _, pool := range profile.Pools {
		if _, ok := poolTickets[pool.Name]; !ok {
			poolTickets[pool.Name] = nil
		}

		for _, ticket := range tickets {
			if pool.In(ticket) {
				poolTickets[pool.Name] = append(poolTickets[pool.Name], ticket)
			}
		}
	}

	return poolTickets, nil
}

func (u *matchUsecase) evaluateMatches(ctx context.Context, matches entity.Matches) (entity.Matches, *errs.Error) {
	if u.evaluator == nil {
		return matches, nil
	}

	evaluatedMatchIDs, err := u.evaluator.Evaluate(ctx, matches)
	if err != nil {
		return nil, entity.ErrMatchEvaluationFailed.WithCause(err)
	}

	evaluatedMatches, _ := matches.SplitByIDs(evaluatedMatchIDs)
	return evaluatedMatches, nil
}

func (u *matchUsecase) assign(ctx context.Context, matches entity.Matches) *errs.Error {
	var ticketIDsToRelease []string
	defer func() {
		if len(ticketIDsToRelease) > 0 {
			_ = u.pendingTicketRepository.ReleaseTickets(ctx, ticketIDsToRelease)
		}
	}()

	asgs, err := u.assigner.Assign(ctx, matches)
	if err != nil {
		ticketIDsToRelease = append(ticketIDsToRelease, matches.TicketIDs()...)
		return entity.ErrMatchAssignFailed.WithCause(err)
	}

	if len(asgs) > 0 {
		notAssigned, err := u.assignerService.AssignTickets(ctx, asgs)
		ticketIDsToRelease = append(ticketIDsToRelease, notAssigned...)
		if err != nil {
			return entity.ErrMatchAssignFailed.WithCause(err)
		}
	}

	return nil
}

func filterUnmatchedTicketIDs(allTickets entity.Tickets, matchedTicketIDs []string) []string {
	matchedSet := lo.SliceToMap(matchedTicketIDs, func(id string) (string, struct{}) {
		return id, struct{}{}
	})

	return lo.FilterMap(allTickets, func(ticket *entity.Ticket, _ int) (string, bool) {
		_, matched := matchedSet[ticket.ID]
		return ticket.ID, !matched
	})
}
