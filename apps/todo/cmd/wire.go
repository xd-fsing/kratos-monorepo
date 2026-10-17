//go:build wireinject
// +build wireinject

// 构建标签保证该桩文件不会进入最终构建。

package main

import (
	"log/slog"

	"kratos-monorepo/apps/todo/internal/conf"
	"kratos-monorepo/apps/todo/internal/data"
	"kratos-monorepo/apps/todo/internal/server"
	"kratos-monorepo/apps/todo/internal/service"

	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

// wireApp 初始化 kratos 应用。
func wireApp(*conf.Server, *conf.Data, *conf.Client, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, service.ProviderSet, newApp))
}
