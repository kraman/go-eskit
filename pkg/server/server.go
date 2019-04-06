package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/kraman/grpc-ms/pkg/sync"
	"github.com/kraman/grpc-ms/pkg/tracer"

	etcdv3 "github.com/coreos/etcd/clientv3"
	etcdconcurrency "github.com/coreos/etcd/clientv3/concurrency"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/serialx/hashring"
	"github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Service interface {
	Name() string
	RegisterHTTPHandler(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error)
	RegisterGRPCServer(ctx context.Context, grpcServer *grpc.Server)
}

type Server interface {
	RegisterService(s Service)
	ListenAndServe(ctx context.Context, endpoint string) error
}

func New(serviceName, podIP string, numShards int) Server {
	return &server{
		services:      []Service{},
		serviceName:   serviceName,
		podIP:         podIP,
		etcdEndpoints: []string{"localhost:2379"},
		numShards:     numShards,
	}
}

type server struct {
	services      []Service
	serviceName   string
	podIP         string
	etcdEndpoints []string
	numShards     int
	ownedShards   map[int]*sync.Semaphore
}

func (s *server) RegisterService(svc Service) {
	s.services = append(s.services, svc)
}

func (s *server) ListenAndServe(ctx context.Context, port string) error {
	etcd, err := etcdv3.New(etcdv3.Config{
		Endpoints:   s.etcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return errors.Wrapf(err, "unable to connect to etcd")
	}
	leaseResp, err := etcd.Lease.Grant(ctx, 2)
	if err != nil {
		return errors.Wrapf(err, "unable to grant pod lease")
	}
	leaseID := leaseResp.ID

	etcdSess, err := etcdconcurrency.NewSession(etcd, etcdconcurrency.WithLease(leaseID), etcdconcurrency.WithContext(ctx))
	if err != nil {
		return errors.Wrapf(err, "unable to get etcd session")
	}
	defer etcdSess.Close()
	_, err = etcd.KV.Put(ctx, fmt.Sprintf("/services/%s/members/%s", s.serviceName, s.podIP), net.JoinHostPort(s.podIP, port), etcdv3.WithLease(leaseID))
	if err != nil {
		return errors.Wrapf(err, "unable to register server with etcd")
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		logrus.Fatal(err)
	}

	m := cmux.New(listener)
	grpcListener := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldPrefixSendSettings("content-type", "application/grpc"))
	httpListener := m.Match(cmux.Any())
	gatewayMux := runtime.NewServeMux(runtime.WithMarshalerOption("application/yaml", &yamlMarshaller{}))
	httpHandler := http.Handler(gatewayMux)

	tracer, closer, err := tracer.NewTracer()
	grpcServerOptions := []grpc.ServerOption{}
	grpcClientOptions := []grpc.DialOption{grpc.WithInsecure()}
	if err != nil {
		logrus.Warn(err)
	} else {
		defer closer.Close()
		opentracing.SetGlobalTracer(tracer)
		grpcServerOptions = []grpc.ServerOption{
			grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(tracer)),
			grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(tracer)),
		}
		grpcClientOptions = []grpc.DialOption{
			grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
			grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(tracer)),
		}
		httpHandler = nethttp.Middleware(
			opentracing.GlobalTracer(),
			httpHandler,
			nethttp.MWComponentName(s.serviceName),
		)
	}

	httpServer := &http.Server{Handler: httpHandler}
	grpcServer := grpc.NewServer(grpcServerOptions...)
	reflection.Register(grpcServer)

	for _, svc := range s.services {
		svc.RegisterGRPCServer(ctx, grpcServer)
		if err := svc.RegisterHTTPHandler(ctx, gatewayMux, net.JoinHostPort(s.podIP, port), grpcClientOptions); err != nil {
			return errors.Wrapf(err, "unable to register http handler for service %s", svc.Name())
		}
	}

	g := new(errgroup.Group)
	g.Go(func() error { return grpcServer.Serve(grpcListener) })
	g.Go(func() error { return httpServer.Serve(httpListener) })
	g.Go(func() error { return m.Serve() })
	g.Go(func() error { return s.keepLeaseAlive(ctx, etcd, leaseID) })
	g.Go(func() error { return s.lockShards(ctx, etcd, leaseID) })

	return g.Wait()
}

func (s *server) keepLeaseAlive(ctx context.Context, etcd *etcdv3.Client, leaseID etcdv3.LeaseID) error {
	t := time.Tick(time.Second)
	for {
		select {
		case <-t:
			if _, err := etcd.Lease.KeepAliveOnce(ctx, leaseID); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *server) lockShards(ctx context.Context, etcd *etcdv3.Client, leaseID etcdv3.LeaseID) error {
	t := time.Tick(time.Second)
	s.ownedShards = map[int]*sync.Semaphore{}

	for {
		select {
		case <-t:
			members, err := s.getPeers(ctx, etcd)
			if err != nil {
				logrus.Error(err)
				break
			}
			ring := hashring.New(members)
			serverBuckets := map[string][]int{}
			for i := 0; i < s.numShards; i++ {
				s, _ := ring.GetNode(strconv.Itoa(i))
				if serverBuckets[s] == nil {
					serverBuckets[s] = []int{}
				}
				serverBuckets[s] = append(serverBuckets[s], i)
			}
			logrus.Println(serverBuckets)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *server) getPeers(ctx context.Context, etcd *etcdv3.Client) (members []string, err error) {
	members = []string{}
	resp, err := etcd.Get(ctx, fmt.Sprintf("/services/%s/members", s.serviceName), etcdv3.WithPrefix(), etcdv3.WithSort(etcdv3.SortByKey, etcdv3.SortAscend))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve members")
	}
	if len(resp.Kvs) <= 0 {
		return
	}
	for _, kv := range resp.Kvs {
		members = append(members, string(kv.Value))
	}
	lastKey := string(resp.Kvs[len(resp.Kvs)-1].Key)

	for resp.More {
		resp, err = etcd.Get(ctx, lastKey, etcdv3.WithPrefix(), etcdv3.WithSort(etcdv3.SortByKey, etcdv3.SortAscend), etcdv3.WithFromKey())
		if err != nil {
			return nil, errors.Wrapf(err, "unable to retrieve members")
		}
		if len(resp.Kvs) <= 0 {
			return
		}
		for _, kv := range resp.Kvs {
			members = append(members, string(kv.Value))
		}
		lastKey = string(resp.Kvs[len(resp.Kvs)-1].Key)
	}
	return
}
