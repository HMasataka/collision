package persistence

import (
	"sync"

	"github.com/HMasataka/collision/domain/driver"
	"github.com/HMasataka/collision/domain/repository"
	"github.com/redis/rueidis"
)

var (
	container     *repository.RepositoryContainer
	containerOnce sync.Once
)

func NewRepositoryOnce(
	client rueidis.Client,
	lockerDriver driver.LockerDriver,
) *repository.RepositoryContainer {
	containerOnce.Do(func() {
		container = newRepository(client, lockerDriver)
	})

	return container
}

func newRepository(
	client rueidis.Client,
	lockerDriver driver.LockerDriver,
) *repository.RepositoryContainer {
	return &repository.RepositoryContainer{
		TicketRepository:        NewTicketRepository(client),
		TicketIDRepository:      NewTicketIDRepository(client),
		PendingTicketRepository: NewPendingTicketRepository(client, lockerDriver),
	}
}
