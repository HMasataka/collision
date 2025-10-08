package collision

import (
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
)

const DefaultLockTTL = 1000 * time.Millisecond

func NewClient() (rueidis.Client, error) {
	client, err := rueidis.NewClient(
		rueidis.ClientOption{
			InitAddress:  []string{"127.0.0.1:6379"},
			Password:     "", // no password set
			DisableCache: true,
		},
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func NewLocker() (rueidislock.Locker, error) {
	locker, err := rueidislock.NewLocker(
		rueidislock.LockerOption{
			ClientOption: rueidis.ClientOption{
				InitAddress:  []string{"127.0.0.1:6379"},
				Password:     "", // no password set
				DisableCache: true,
			},
			KeyMajority:    1,    // Use KeyMajority=1 if you have only one Redis instance. Also make sure that all your `Locker`s share the same KeyMajority.
			NoLoopTracking: true, // Enable this to have better performance if all your Redis are >= 7.0.5.
		},
	)
	if err != nil {
		return nil, err
	}

	return locker, nil
}
