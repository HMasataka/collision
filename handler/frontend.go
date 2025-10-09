package handler

import (
	"context"

	"github.com/HMasataka/collision/gen/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Frontend struct {
	pb.UnimplementedFrontendServiceServer
}

func (Frontend) CreateTicket(context.Context, *pb.CreateTicketRequest) (*pb.CreateTicketResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateTicket not implemented")
}

func (Frontend) DeleteTicket(context.Context, *pb.DeleteTicketRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteTicket not implemented")
}

func (Frontend) GetTicket(context.Context, *pb.GetTicketRequest) (*pb.Ticket, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTicket not implemented")
}
