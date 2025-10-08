package main

import (
	"context"
	"fmt"

	"github.com/HMasataka/collision"
)

func main() {
	ctx := context.Background()

	client, err := collision.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	query := client.B().Ping().Build()
	if err := client.Do(ctx, query).Error(); err != nil {
		panic(err)
	}

	query = client.B().Set().Key("my-key").Value("initial").Build()
	if err := client.Do(ctx, query).Error(); err != nil {
		panic(err)
	}

	locker, err := collision.NewLocker()
	if err != nil {
		panic(err)
	}

	lockedContext, unlock, err := locker.WithContext(ctx, "my-key")
	if err != nil {
		panic(err)
	}
	defer unlock()

	if err := withoutLock(); err != nil {
		fmt.Println("withoutLock error:", err)
		panic(err)
	}

	query = client.B().Set().Key("my-key").Value("withlock").Build()
	if err := client.Do(lockedContext, query).Error(); err != nil {
		panic(err)
	}
}

func withoutLock() error {
	ctx := context.Background()

	client, err := collision.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	query := client.B().Set().Key("my-key").Value("withoutlock").Build()
	if err := client.Do(ctx, query).Error(); err != nil {
		return err
	}

	query = client.B().Get().Key("my-key").Build()
	res, err := client.Do(ctx, query).ToString()
	if err != nil {
		return err
	}

	fmt.Println("withoutLock result:", res)

	return nil
}
