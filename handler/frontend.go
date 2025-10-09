package handler

import (
	"context"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/gen/pb"
	"github.com/HMasataka/collision/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Frontend struct {
	ticketUsecase usecase.TicketUsecase
	assignUsecase usecase.AssignUsecase

	pb.UnimplementedFrontendServiceServer
}

func NewFrontend(
	ticketUsecase usecase.TicketUsecase,
) *Frontend {
	return &Frontend{
		ticketUsecase: ticketUsecase,
	}
}

func (h Frontend) CreateTicket(ctx context.Context, req *pb.CreateTicketRequest) (*pb.CreateTicketResponse, error) {
	searchFields := ToSearchFields(req.GetSearchFields())

	res, err := h.ticketUsecase.CreateTicket(ctx, searchFields, req.GetExtensions())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create ticket: %v", err)
	}

	return &pb.CreateTicketResponse{
		Id:         res.ID,
		CreateTime: timestamppb.New(res.CreatedAt),
	}, nil
}

func (Frontend) DeleteTicket(context.Context, *pb.DeleteTicketRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteTicket not implemented")
}

func (Frontend) GetTicket(context.Context, *pb.GetTicketRequest) (*pb.Ticket, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTicket not implemented")
}

func (h Frontend) WatchAssignments(req *pb.WatchAssignmentsRequest, stream pb.FrontendService_WatchAssignmentsServer) error {
	ticketID := req.GetTicketId()

	if err := h.assignUsecase.Watch(stream.Context(), ticketID, func(assignment *entity.Assignment) error {
		if assignment == nil {
			return nil
		}

		pbAssignment := &pb.Assignment{
			Connection: assignment.Connection,
			Extensions: assignment.Extensions,
		}

		if err := stream.Send(&pb.WatchAssignmentsResponse{
			Assignment: pbAssignment,
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to watch assignments: %v", err)
	}

	return status.Errorf(codes.Unimplemented, "method WatchAssignments not implemented")
}
