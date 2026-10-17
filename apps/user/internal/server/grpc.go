package server

import (
	v1 "kratos-monorepo/api/user/v1"
	"kratos-monorepo/apps/user/internal/conf"
	"kratos-monorepo/apps/user/internal/service"
	pkgsrv "kratos-monorepo/pkg/server"

	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer 创建一个 gRPC 服务。
func NewGRPCServer(c *conf.Server, user *service.UserService) *grpc.Server {
	return pkgsrv.NewGRPCServer(c.Grpc.Network, c.Grpc.Addr, c.Grpc.Timeout.AsDuration(),
		func(srv *grpc.Server) { v1.RegisterUserServiceServer(srv, user) })
}
