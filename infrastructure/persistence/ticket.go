package persistence

import (
	"context"
	"encoding/json"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
)

type ticketRepository struct {
	// NOTE 全体で共通の実態を持つ
	locker rueidislock.Locker
	client rueidis.Client
}

func NewTicketRepository(
	locker rueidislock.Locker,
	client rueidis.Client,
) repository.TicketRepository {
	return &ticketRepository{
		locker: locker,
		client: client,
	}
}

func (r *ticketRepository) getKey(id string) string {
	return id
}

func (r *ticketRepository) WithLock(ctx context.Context, id string, fn func(ctx context.Context) error) error {
	lockedCtx, unlock, err := r.locker.WithContext(ctx, r.getKey(id))
	if err != nil {
		return err
	}
	defer unlock()

	return fn(lockedCtx)
}

func (r *ticketRepository) Insert(ctx context.Context, target *entity.Ticket) error {
	data, err := json.Marshal(target)
	if err != nil {
		return err
	}

	query := r.client.B().Set().Key(r.getKey(target.ID)).Value(rueidis.BinaryString(data)).Build()
	return r.client.Do(ctx, query).Error()
}

func (r *ticketRepository) Find(ctx context.Context, id string) (*entity.Ticket, error) {
	query := r.client.B().Get().Key(r.getKey(id)).Build()
	data, err := r.client.Do(ctx, query).AsBytes()
	if err != nil {
		return nil, err
	}

	var ticket entity.Ticket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return nil, err
	}

	return &ticket, nil
}

func (r *ticketRepository) Delete(ctx context.Context, target *entity.Ticket) error {
	query := r.client.B().Del().Key(r.getKey(target.ID)).Build()
	if err := r.client.Do(ctx, query).Error(); err != nil {
		return err
	}

	return nil
}
