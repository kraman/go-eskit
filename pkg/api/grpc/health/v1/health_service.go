package v1

import (
	context "context"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/kraman/grpc-ms/pkg/server"
	grpc "google.golang.org/grpc"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

type HealthReporter interface {
	server.Service
	Status() grpc_health_v1.HealthCheckResponse_ServingStatus
}

type HealthService interface {
	server.Service
	RegisterService(s HealthReporter)
}

type healthService struct {
	services []HealthReporter
}

func New() HealthService {
	h := &healthService{services: []HealthReporter{}}
	h.services = append(h.services, h)
	return h
}

func (h *healthService) Name() string {
	return "health"
}

func (h *healthService) RegisterService(s HealthReporter) {
	h.services = append(h.services, s)
}

func (h *healthService) RegisterHTTPHandler(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
	// return RegisterHealthHandlerFromEndpoint(ctx, mux, endpoint, opts)
	return nil
}

func (h *healthService) RegisterGRPCServer(ctx context.Context, grpcServer *grpc.Server) {
	grpc_health_v1.RegisterHealthServer(grpcServer, h)
}

func (h *healthService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (resp *grpc_health_v1.HealthCheckResponse, err error) {
	overallStatus := grpc_health_v1.HealthCheckResponse_SERVING
	for _, svc := range h.services {
		if svc.Status() != overallStatus {
			overallStatus = svc.Status()
		}
		if svc.Name() == req.GetService() {
			return &grpc_health_v1.HealthCheckResponse{Status: svc.Status()}, nil
		}
	}
	if req.Service == "" {
		return &grpc_health_v1.HealthCheckResponse{Status: overallStatus}, nil
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVICE_UNKNOWN}, nil
}

func (h *healthService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	t := time.Tick(time.Second * 5)
	for {
		select {
		case <-t:
			resp, err := h.Check(srv.Context(), req)
			if err != nil {
				return err
			}
			return srv.Send(resp)
		}
	}
}

func (h *healthService) Status() grpc_health_v1.HealthCheckResponse_ServingStatus {
	return grpc_health_v1.HealthCheckResponse_SERVING
}
