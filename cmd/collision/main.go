package main

import (
	"context"

	"github.com/HMasataka/collision"
)

func main() {
	ctx := context.Background()

	client, err := collision.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	if _, err := client.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	println("Connected to Redis successfully")
}
