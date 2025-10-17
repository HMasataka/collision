package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/gen/pb"
	"github.com/HMasataka/collision/handler"
	"github.com/HMasataka/collision/infrastructure"
	"github.com/HMasataka/collision/infrastructure/persistence"
	"github.com/HMasataka/collision/usecase"
	"google.golang.org/grpc"
)

func getListener() (net.Listener, error) {
	port := "31080"
	address := fmt.Sprintf("127.0.0.1:%v", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	fmt.Println("Listening on", address)

	return listener, nil
}

var matchProfile = &entity.MatchProfile{
	Name: "simple-1vs1",
	Pools: []*entity.Pool{
		{Name: "test-pool"},
	},
}

func di() (usecase.MatchUsecase, usecase.TicketUsecase, usecase.AssignUsecase, error) {
	redisClient, err := infrastructure.NewClient()
	if err != nil {
		return nil, nil, nil, err
	}

	redisLocker, err := infrastructure.NewLocker()
	if err != nil {
		return nil, nil, nil, err
	}

	ticketRepository := persistence.NewTicketRepository(redisClient, redisLocker)

	matchFunctions := map[*entity.MatchProfile]entity.MatchFunction{
		matchProfile: usecase.NewSimple1vs1MatchFunction(),
	}

	assigner := usecase.NewRandomAssigner()

	matchUsecase := usecase.NewMatchUsecase(matchFunctions, assigner, nil, ticketRepository)
	ticketUsecase := usecase.NewTicketUsecase(ticketRepository)
	assignUsecase := usecase.NewAssignUsecase(ticketRepository)

	return matchUsecase, ticketUsecase, assignUsecase, nil
}

func main() {
	listener, err := getListener()
	if err != nil {
		panic(err)
	}

	matchUsecase, ticketUsecase, assignUsecase, err := di()
	if err != nil {
		panic(err)
	}

	go func() {
		ctx := context.Background()
		if err := startMatchLoop(ctx, matchUsecase); err != nil {
			panic(err)
		}
	}()

	grpcServer := grpc.NewServer()

	frontendHandler := handler.NewFrontend(ticketUsecase, assignUsecase)
	pb.RegisterFrontendServiceServer(grpcServer, frontendHandler)

	if err := grpcServer.Serve(listener); err != nil {
		panic(err)
	}
}

func startMatchLoop(ctx context.Context, matchUsecase usecase.MatchUsecase) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// The processing tick is not interrupted even if the context is canceled.
			// However, the next tick will not be executed, which is a graceful shutdown process.
			if err := matchUsecase.Exec(context.Background(), nil, nil); err != nil {
				fmt.Printf("failed to exec match usecase: %+v", err)
			}
		}

	}
}
