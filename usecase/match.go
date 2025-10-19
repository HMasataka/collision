package usecase

import (
	"context"
	"fmt"
	"sync"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/HMasataka/collision/domain/service"
	"golang.org/x/sync/errgroup"
)

type MatchUsecase interface {
	Exec(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) error
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

func (u *matchUsecase) Exec(ctx context.Context, searchFields *entity.SearchFields, extensions []byte) error {
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

	unmatchedTicketIDs := filterUnmatchedTicketIDs(activeTickets, matches)
	if len(unmatchedTicketIDs) > 0 {
		if err := u.pendingTicketRepository.ReleaseTickets(ctx, unmatchedTicketIDs); err != nil {
			return fmt.Errorf("failed to release unmatched tickets: %w", err)
		}
	}

	if len(matches) > 0 {
		if err := u.assign(ctx, matches); err != nil {
			return err
		}
	}

	return nil

}

func (u *matchUsecase) fetchActiveTickets(ctx context.Context, limit int64) ([]*entity.Ticket, error) {
	activeTicketIDs, err := u.ticketService.GetActiveTicketIDs(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active ticket IDs: %w", err)
	}
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}

	tickets, deletedTicketIDs, err := u.ticketRepository.GetTickets(ctx, activeTicketIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active tickets: %w", err)
	}

	if len(deletedTicketIDs) > 0 {
		if err := u.ticketService.DeleteIndexTickets(ctx, deletedTicketIDs); err != nil {
			return nil, fmt.Errorf("failed to delete index tickets: %w", err)
		}
	}

	return tickets, nil
}

func (u *matchUsecase) makeMatches(ctx context.Context, activeTickets []*entity.Ticket) ([]*entity.Match, error) {
	u.mutex.RLock()
	mmfs := u.matchFunctions
	u.mutex.RUnlock()

	resCh := make(chan []*entity.Match, len(mmfs))
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
		return nil, err
	}

	close(resCh)

	var totalMatches []*entity.Match
	for matches := range resCh {
		totalMatches = append(totalMatches, matches...)
	}

	return totalMatches, nil
}

func filterTickets(profile *entity.MatchProfile, tickets []*entity.Ticket) (map[string][]*entity.Ticket, error) {
	poolTickets := map[string][]*entity.Ticket{}
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

func (u *matchUsecase) evaluateMatches(ctx context.Context, matches []*entity.Match) ([]*entity.Match, error) {
	if u.evaluator == nil {
		return matches, nil
	}

	evaluatedMatches := make([]*entity.Match, 0, len(matches))

	evaluatedMatchIDs, err := u.evaluator.Evaluate(ctx, matches)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate matches: %w", err)
	}

	evaluatedMap := map[string]struct{}{}
	for _, evaluatedID := range evaluatedMatchIDs {
		evaluatedMap[evaluatedID] = struct{}{}
	}

	for _, match := range matches {
		if _, ok := evaluatedMap[match.MatchID]; ok {
			evaluatedMatches = append(evaluatedMatches, match)
		}
	}

	return evaluatedMatches, nil
}

func (u *matchUsecase) assign(ctx context.Context, matches []*entity.Match) error {
	var ticketIDsToRelease []string
	defer func() {
		if len(ticketIDsToRelease) > 0 {
			_ = u.pendingTicketRepository.ReleaseTickets(ctx, ticketIDsToRelease)
		}
	}()

	asgs, err := u.assigner.Assign(ctx, matches)
	if err != nil {
		ticketIDsToRelease = append(ticketIDsToRelease, ticketIDsFromMatches(matches)...)
		return fmt.Errorf("failed to assign matches: %w", err)
	}
	if len(asgs) > 0 {
		notAssigned, err := u.assignerService.AssignTickets(ctx, asgs)
		ticketIDsToRelease = append(ticketIDsToRelease, notAssigned...)
		if err != nil {
			return fmt.Errorf("failed to assign tickets: %w", err)
		}
	}
	return nil
}

func filterUnmatchedTicketIDs(allTickets []*entity.Ticket, matches []*entity.Match) []string {
	matchedTickets := map[string]struct{}{}
	for _, match := range matches {
		for _, ticketID := range match.Tickets.IDs() {
			matchedTickets[ticketID] = struct{}{}
		}
	}

	var unmatchedTicketIDs []string
	for _, ticket := range allTickets {
		if _, ok := matchedTickets[ticket.ID]; !ok {
			unmatchedTicketIDs = append(unmatchedTicketIDs, ticket.ID)
		}
	}

	return unmatchedTicketIDs
}

func ticketIDsFromMatches(matches []*entity.Match) []string {
	var ticketIDs []string
	for _, match := range matches {
		for _, ticket := range match.Tickets {
			ticketIDs = append(ticketIDs, ticket.ID)
		}
	}
	return ticketIDs
}
