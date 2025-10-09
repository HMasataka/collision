package main

import (
	"context"
	"fmt"

	"github.com/HMasataka/collision/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getConnection() (*grpc.ClientConn, error) {
	port := "31080"
	address := fmt.Sprintf("127.0.0.1:%v", port)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	fmt.Println("Connecting to", address)

	return conn, nil
}

func main() {
	conn, err := getConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ctx := context.Background()

	client := pb.NewFrontendServiceClient(conn)

	response, err := client.CreateTicket(ctx, &pb.CreateTicketRequest{})
	if err != nil {
		panic(err)
	}

	fmt.Println("Ticket ID:", response.Id)
}
