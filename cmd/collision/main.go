package main

import (
	"fmt"
	"net"

	"github.com/HMasataka/collision/gen/pb"
	"github.com/HMasataka/collision/handler"
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

func main() {
	listener, err := getListener()
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterFrontendServiceServer(grpcServer, &handler.Frontend{})

	if err := grpcServer.Serve(listener); err != nil {
		panic(err)
	}
}
