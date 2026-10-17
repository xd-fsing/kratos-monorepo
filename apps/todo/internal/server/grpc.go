package server

import (
	v1 "kratos-monorepo/api/todo/v1"
	"kratos-monorepo/apps/todo/internal/conf"
	"kratos-monorepo/apps/todo/internal/service"
	pkgsrv "kratos-monorepo/pkg/server"

	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer 创建一个 gRPC 服务。
func NewGRPCServer(c *conf.Server, todo *service.TodoService) *grpc.Server {
	return pkgsrv.NewGRPCServer(c.Grpc.Network, c.Grpc.Addr, c.Grpc.Timeout.AsDuration(),
		func(srv *grpc.Server) { v1.RegisterTodoServiceServer(srv, todo) })
}
