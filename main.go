package main

import (
	"context"
	"google.golang.org/grpc/reflection"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/features/proto/echo"
	"google.golang.org/grpc/health/grpc_health_v1"
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

// Echo returns the same message sent by the client.
func (e *EchoService) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	return &echo.EchoResponse{Message: req.GetMessage()}, nil
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	// メインのサービスの登録
	echoServer := &EchoService{}
	echo.RegisterEchoServer(server, echoServer)

	// ヘルスサービスの登録
	healthServer := &HealthService{}
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	// grpcurl で proto ファイルを指定せずにサービスを呼び出すため
	reflection.Register(server)

	log.Println("gRPC server is running on port 50051")
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	// 商用サービスならグレースフルシャットダウンなどの実装が必要
}
