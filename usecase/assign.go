package usecase

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/service"
	"github.com/HMasataka/errs"
	"github.com/sethvargo/go-retry"
)

type AssignUsecase interface {
	Watch(ctx context.Context, ticketID string, onAssignmentChanged func(*entity.Assignment) error) *errs.Error
}

type assignUsecase struct {
	assignerService service.AssignerService
}

func NewAssignUsecase(
	assignerService service.AssignerService,
) AssignUsecase {
	return &assignUsecase{
		assignerService: assignerService,
	}
}

const (
	watchAssignmentInterval = 100 * time.Millisecond
)

func (u *assignUsecase) Watch(ctx context.Context, ticketID string, onAssignmentChanged func(*entity.Assignment) error) *errs.Error {
	var prev *entity.Assignment

	backoff := newWatchAssignmentBackoff()

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		assignment, err := u.assignerService.GetAssignment(ctx, ticketID)
		if err != nil {
			if errors.Is(err, entity.ErrAssignmentNotFound) {
				return retry.RetryableError(err)
			}
			return err
		}

		if (prev == nil && assignment != nil) || !reflect.DeepEqual(prev, assignment) {
			prev = assignment
			if err := onAssignmentChanged(assignment); err != nil {
				return err
			}
		}

		return retry.RetryableError(errs.New("assignment unchanged"))
	}); err != nil {
		return entity.ErrAssignmentWatchFailed.WithCause(err)
	}
	return nil
}

func newWatchAssignmentBackoff() retry.Backoff {
	return retry.NewConstant(watchAssignmentInterval)
}
