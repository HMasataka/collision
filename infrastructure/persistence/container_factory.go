package persistence

import (
	"sync"

	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
)

var (
	container     *repository.RepositoryContainer
	containerOnce sync.Once
)

func NewRepositoryOnce(
	client rueidis.Client,
	locker rueidislock.Locker,
) *repository.RepositoryContainer {
	containerOnce.Do(func() {
		container = newRepository(client, locker)
	})

	return container
}

func newRepository(
	client rueidis.Client,
	locker rueidislock.Locker,
) *repository.RepositoryContainer {
	return &repository.RepositoryContainer{
		TicketRepository:   NewTicketRepository(client, locker),
		TicketIDRepository: NewTicketIDRepository(client, locker),
	}
}
