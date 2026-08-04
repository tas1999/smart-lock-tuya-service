package password

import (
	"context"
	"strings"

	pb "github.com/tas1999/smart-lock-tuya-service/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	pb.UnimplementedPasswordServiceServer
	Client *OnlineClient
}

func (s *GRPCServer) CreateTemporaryPassword(ctx context.Context, req *pb.CreateTemporaryPasswordRequest) (*pb.CreateTemporaryPasswordResponse, error) {
	if s.Client == nil {
		return nil, status.Error(codes.Unavailable, "password client not configured")
	}
	res, err := s.Client.Create(ctx, req.GetDeviceId(), req.GetName(), req.GetEffectiveTime(), req.GetInvalidTime(), strings.TrimSpace(req.GetPassword()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create temporary password: %v", err)
	}
	return &pb.CreateTemporaryPasswordResponse{
		Password:      res.Password,
		PasswordId:    res.PasswordID,
		EffectiveTime: res.Effective,
		InvalidTime:   res.Invalid,
	}, nil
}

func (s *GRPCServer) DeleteTemporaryPassword(ctx context.Context, req *pb.DeleteTemporaryPasswordRequest) (*pb.DeleteTemporaryPasswordResponse, error) {
	if s.Client == nil {
		return nil, status.Error(codes.Unavailable, "password client not configured")
	}
	if err := s.Client.Delete(ctx, req.GetDeviceId(), req.GetPasswordId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete temporary password: %v", err)
	}
	return &pb.DeleteTemporaryPasswordResponse{Success: true}, nil
}
