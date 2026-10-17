// Package server 构造所有应用共用的 HTTP/gRPC 服务与公共中间件栈。
// 必须保持服务无关：不得 import apps/* 或 api/*。
package server

import (
	"time"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"

	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer 以共享中间件栈构建 HTTP 服务，并让调用方注册自己的服务。
// network/addr 为空或 timeout 为 0 时，不设置对应的服务选项。
func NewHTTPServer(network, addr string, timeout time.Duration, register func(*http.Server)) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			validate.Validator(func(req any) error {
				if msg, ok := req.(proto.Message); ok {
					if err := fieldbehavior.ValidateRequiredFields(msg); err != nil {
						return err
					}
				}
				return nil
			}),
		),
	}
	if network != "" {
		opts = append(opts, http.Network(network))
	}
	if addr != "" {
		opts = append(opts, http.Address(addr))
	}
	if timeout > 0 {
		opts = append(opts, http.Timeout(timeout))
	}
	srv := http.NewServer(opts...)
	register(srv)
	return srv
}

// NewGRPCServer 以共享中间件栈构建 gRPC 服务，并让调用方注册自己的服务。
// network/addr 为空或 timeout 为 0 时，不设置对应的服务选项。
func NewGRPCServer(network, addr string, timeout time.Duration, register func(*grpc.Server)) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
	}
	if network != "" {
		opts = append(opts, grpc.Network(network))
	}
	if addr != "" {
		opts = append(opts, grpc.Address(addr))
	}
	if timeout > 0 {
		opts = append(opts, grpc.Timeout(timeout))
	}
	srv := grpc.NewServer(opts...)
	register(srv)
	return srv
}
