package main

import (
	"context"
	"errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/features/proto/echo"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
)

// HealthService implements the gRPC health checking service.
type HealthService struct{}

// Check メソッドの実装
func (h *HealthService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// 商用サービスなら、ここでデータベースに接続できるかなどをチェックする
	return &grpc_health_v1.HealthCheckResponse{
		// サーバーが正常な状態でなければ NOT_SERVING を返すようにする
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch メソッドの実装
func (h *HealthService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	// ここでは常に SERVING と NOT_SERVING を連続して返し続けてい
	// 本来はサーバーの状態に応じてステータスが変わったときだけ返すようにする
	for {
		if err := srv.Send(&grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}); err != nil {
			return err
		}

		if err := srv.Send(&grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}); err != nil {
			return err
		}
	}
}

// EchoService implements the Echo service defined in the proto file.
type EchoService struct {
	echo.UnimplementedEchoServer
}

// UnaryEcho returns the same message sent by the client.
func (e *EchoService) UnaryEcho(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	return &echo.EchoResponse{Message: req.GetMessage()}, nil
}

func setupOpenTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
	)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	return shutdown, nil
}

func main() {
	ctx := context.Background()

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	shutdown, err := setupOpenTelemetry(ctx)
	if err != nil {
		log.Fatalf("error setting up OpenTelemetry: %v", err)
	}

	server := grpc.NewServer(
		grpc.StatsHandler(
			otelgrpc.NewServerHandler(
				otelgrpc.WithFilter(
					filters.Not(filters.HealthCheck()),
				),
			),
		),
	)

	// メインのサービスの登録
	echoServer := &EchoService{}
	echo.RegisterEchoServer(server, echoServer)

	// ヘルスサービスの登録
	healthServer := &HealthService{}
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	// grpcurl で proto ファイルを指定せずにサービスを呼び出すため
	reflection.Register(server)

	log.Println("gRPC server is running on port 50051")
	if err := errors.Join(server.Serve(listener), shutdown(ctx)); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
