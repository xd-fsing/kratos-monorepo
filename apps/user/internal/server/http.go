package server

import (
	v1 "kratos-monorepo/api/user/v1"
	"kratos-monorepo/apps/user/internal/conf"
	"kratos-monorepo/apps/user/internal/service"
	pkgsrv "kratos-monorepo/pkg/server"

	"github.com/go-kratos/kratos/v3/transport/http"
)

// NewHTTPServer 创建一个 HTTP 服务。
func NewHTTPServer(c *conf.Server, user *service.UserService) *http.Server {
	return pkgsrv.NewHTTPServer(c.Http.Network, c.Http.Addr, c.Http.Timeout.AsDuration(),
		func(srv *http.Server) { v1.RegisterUserServiceHTTPServer(srv, user) })
}
