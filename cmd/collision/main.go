package main

import (
	"context"
	"fmt"
	"time"

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

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, ul, err := locker.WithContext(ctx, "my-key")
	if err != nil {
		panic(err)
	}
	defer ul()

	_, unlock, err := locker.WithContext(timeoutCtx, "my-key")
	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Println("ロック取得がタイムアウトしました")
			return
		}
		fmt.Println("ロック取得に失敗しました:", err)
		return
	}
	defer unlock()

	fmt.Println("ロックを取得しました")
}
