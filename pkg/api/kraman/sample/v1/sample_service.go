package v1

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

func New() *sampleService {
	s := &sampleService{}
	return s
}

type sampleService struct{}

func (s *sampleService) Name() string {
	return "lur"
}

func (s *sampleService) RegisterHTTPHandler(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
	return RegisterSampleServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
}

func (s *sampleService) RegisterGRPCServer(ctx context.Context, grpcServer *grpc.Server) {
	RegisterSampleServiceServer(grpcServer, s)
}

func (s *sampleService) Status() grpc_health_v1.HealthCheckResponse_ServingStatus {
	return grpc_health_v1.HealthCheckResponse_NOT_SERVING
}
