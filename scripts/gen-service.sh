#!/usr/bin/env bash
# 在 apps/<app> 下生成一个新服务的骨架：入口、wire 注入器、配置、ent 管道，
# 以及轻薄的 server/service/data 层。生成结果开箱即可编译运行；随后在
# api/<domain>/v1/ 下定义你的 proto，并按 CLAUDE.md 中的加服务清单继续。
#
# 用法：scripts/gen-service.sh <app>   （在仓库根目录执行）
set -euo pipefail

cd "$(dirname "$0")/.."

app="${1:-}"
if [ -z "$app" ]; then
	echo "usage: $0 <app>" >&2
	exit 1
fi

root="apps/$app"
if [ -d "$root" ]; then
	echo "$root already exists" >&2
	exit 1
fi

# 下一对空闲端口：已有服务占用 8000+N / 9000+N。
count=$(find apps -mindepth 1 -maxdepth 1 -type d | wc -l)
http_port=$((8000 + count))
grpc_port=$((9000 + count))
upper=$(printf '%s' "$app" | tr '[:lower:]' '[:upper:]')

mkdir -p "$root/cmd" "$root/configs" "$root/internal/conf" \
	"$root/internal/server" "$root/internal/service" \
	"$root/internal/data/ent/schema"

cat > "$root/configs/config.yaml" <<EOF
server:
  http:
    addr: 0.0.0.0:$http_port
    timeout: 1s
  grpc:
    addr: 0.0.0.0:$grpc_port
    timeout: 1s
data:
  database:
    driver: mysql
    # 每个服务独立的数据库与环境变量：各服务持有自己的存储，一个环境变量
    # 不可能把两个服务翻到同一个库上。
    source: \${${upper}_DATABASE_SOURCE:root:root@tcp(127.0.0.1:3306)/test_${app}?timeout=5s&parseTime=True&loc=Local&charset=utf8mb4}
    # 开发便利项。生产环境两者都保持 false，并把 schema 变更作为单独评审的
    # 步骤执行。
    debug: true
    auto_migrate: true
  redis:
    addr: \${REDIS_ADDR:127.0.0.1:6379}
    read_timeout: 0.2s
    write_timeout: 0.2s
EOF

cat > "$root/internal/conf/conf.proto" <<EOF
syntax = "proto3";
package $app.conf;

option go_package = "kratos-monorepo/apps/$app/internal/conf;conf";

import "google/protobuf/duration.proto";

message Bootstrap {
  Server server = 1;
  Data data = 2;
}

message Server {
  message HTTP {
    string network = 1;
    string addr = 2;
    google.protobuf.Duration timeout = 3;
  }
  message GRPC {
    string network = 1;
    string addr = 2;
    google.protobuf.Duration timeout = 3;
  }
  HTTP http = 1;
  GRPC grpc = 2;
}

message Data {
  message Database {
    // 注册到 database/sql 的驱动名，例如 "mysql"。
    string driver = 1;
    // 驱动专用的 DSN。凭据不要进入版本控制。
    string source = 2;
    // 打印每条生成的 SQL。仅限开发环境。
    bool debug = 3;
    // 启动时按 ent schema 建表或更新表。生产环境保持 false，并把迁移
    // 作为单独评审的步骤执行。
    bool auto_migrate = 4;
  }
  message Redis {
    string network = 1;
    string addr = 2;
    google.protobuf.Duration read_timeout = 3;
    google.protobuf.Duration write_timeout = 4;
  }
  Database database = 1;
  Redis redis = 2;
}
EOF

cat > "$root/internal/data/data.go" <<EOF
package data

import (
	"context"

	"kratos-monorepo/apps/$app/internal/conf"
	"kratos-monorepo/apps/$app/internal/data/ent"

	"github.com/go-kratos/kratos/v3/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
)

// ProviderSet 是 data 层的提供者集合。store 构造函数就绪后加入这里
// （见 CLAUDE.md 的加服务清单）。
var ProviderSet = wire.NewSet(NewData)

// Data 持有各 store 共用的长生命周期存储客户端。
type Data struct {
	db *ent.Client
}

// NewData 打开数据库客户端，并连同清理函数一起返回。
func NewData(c *conf.Data) (*Data, func(), error) {
	dc := c.GetDatabase()
	db, err := ent.Open(dc.GetDriver(), dc.GetSource())
	if err != nil {
		return nil, nil, err
	}
	if dc.GetDebug() {
		db = db.Debug()
	}
	// 自动迁移仅为本地开发提供便利。生产环境保持关闭，并把 schema 变更
	// 作为单独评审的步骤执行。
	if dc.GetAutoMigrate() {
		if err := db.Schema.Create(context.Background()); err != nil {
			db.Close()
			return nil, nil, err
		}
	}
	cleanup := func() {
		log.Info("closing the data resources")
		if err := db.Close(); err != nil {
			log.Error("failed closing the database", "err", err)
		}
	}
	return &Data{db: db}, cleanup, nil
}
EOF

cat > "$root/internal/data/ent/generate.go" <<EOF
package ent

// 按需给 schema 增加 --feature 参数，例如需要 OnConflict 支持时加
// sql/upsert，需要原生 SQL 时加 sql/execquery。
//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema
EOF

cat > "$root/internal/data/ent/schema/mixin.go" <<EOF
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"
)

// IDMixin 为 schema 添加标准的应用侧生成 UUID 主键。
type IDMixin struct {
	mixin.Schema
}

// Fields of the IDMixin.
func (IDMixin) Fields() []ent.Field {
	return []ent.Field{
		// 不指定 SchemaType：ent 会按方言映射 TypeUUID（MySQL 为 char(36)、
		// Postgres 为原生 uuid），因此 Go 侧各处始终是 uuid.UUID，无需
		// 钉死列类型。
		field.UUID("id", uuid.Nil).Default(newUUID).Immutable(),
	}
}

// newUUID 返回应用侧生成的 UUIDv7。
func newUUID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// TimeMixin 为 schema 添加标准的审计时间戳。
type TimeMixin struct {
	mixin.Schema
}

// Fields of the TimeMixin.
func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").Immutable().Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Indexes of the TimeMixin.
func (TimeMixin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("updated_at"),
	}
}
EOF

cat > "$root/internal/data/ent/schema/example.go" <<EOF
package schema

import (
	"entgo.io/ent"
)

// Example 是占位实体，让 ent 客户端能够生成、服务能够开箱运行。
// 换成你自己的 schema 后，执行 `go generate ./apps/$app/internal/data/ent`。
type Example struct {
	ent.Schema
}

// Mixin of the Example.
func (Example) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Example.
func (Example) Fields() []ent.Field {
	return nil
}
EOF

cat > "$root/internal/service/service.go" <<EOF
package service

import "github.com/google/wire"

// ProviderSet 是 service 层的提供者集合。proto 就绪后把服务构造函数加入
// 这里（见 CLAUDE.md 的加服务清单）。
var ProviderSet = wire.NewSet()
EOF

cat > "$root/internal/server/server.go" <<EOF
package server

import (
	"github.com/google/wire"
)

// ProviderSet 是 server 层的提供者集合。
var ProviderSet = wire.NewSet(NewGRPCServer, NewHTTPServer)
EOF

cat > "$root/internal/server/http.go" <<EOF
package server

import (
	"kratos-monorepo/apps/$app/internal/conf"
	pkgsrv "kratos-monorepo/pkg/server"

	"github.com/go-kratos/kratos/v3/transport/http"
)

// NewHTTPServer 创建一个 HTTP 服务。
// proto 就绪后在这里注册你的服务（见 CLAUDE.md 的加服务清单）。
func NewHTTPServer(c *conf.Server) *http.Server {
	return pkgsrv.NewHTTPServer(c.Http.Network, c.Http.Addr, c.Http.Timeout.AsDuration(),
		func(*http.Server) {})
}
EOF

cat > "$root/internal/server/grpc.go" <<EOF
package server

import (
	"kratos-monorepo/apps/$app/internal/conf"
	pkgsrv "kratos-monorepo/pkg/server"

	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer 创建一个 gRPC 服务。
// proto 就绪后在这里注册你的服务（见 CLAUDE.md 的加服务清单）。
func NewGRPCServer(c *conf.Server) *grpc.Server {
	return pkgsrv.NewGRPCServer(c.Grpc.Network, c.Grpc.Addr, c.Grpc.Timeout.AsDuration(),
		func(*grpc.Server) {})
}
EOF

cat > "$root/cmd/main.go" <<EOF
package main

import (
	"flag"
	"log/slog"
	"os"

	"kratos-monorepo/apps/$app/internal/conf"

	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"

	_ "go.uber.org/automaxprocs"
)

// 构建时可通过 -ldflags "-X main.Version=x.y.z" 注入版本号。
var (
	// Name 是编译产物的服务名，可通过 -ldflags "-X main.Name=..." 覆盖。
	Name string = "$app"
	// Version 是编译产物的版本号。
	Version string
	// flagconf 是配置路径参数。
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../configs", "config path, eg: -conf config.yaml")
}

func newApp(logger *slog.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()
	logger := log.NewLogger(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelInfo,
		}),
		log.WithExtractor(tracing.TraceAttrs),
	).With(
		slog.String("service.id", id),
		slog.String("service.name", Name),
		slog.String("service.version", Version),
	)
	log.SetDefault(logger)
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
			env.NewSource("KRATOS"),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bc.Server, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// 启动并等待停止信号。
	if err := app.Run(); err != nil {
		panic(err)
	}
}
EOF

cat > "$root/cmd/wire.go" <<EOF
//go:build wireinject
// +build wireinject

// 构建标签保证该桩文件不会进入最终构建。

package main

import (
	"log/slog"

	"kratos-monorepo/apps/$app/internal/conf"
	"kratos-monorepo/apps/$app/internal/server"

	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

// wireApp 初始化 kratos 应用。
// 首个 store 与 service 就绪后，把 data.ProviderSet + *conf.Data 与
// service.ProviderSet 加进构建（见 CLAUDE.md 的加服务清单）；wire 会
// 拒绝空的或未被使用的提供者集合。
func wireApp(*conf.Server, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, newApp))
}
EOF

# 生成配置桩、ent 客户端与 wire 注入器。此时还没有 api proto：
# 等用户添加之后再执行 make api app=$app。
buf generate --template buf.gen.config.yaml --path "apps/$app/internal/conf"
go generate "./apps/$app/internal/data/ent"
(
	cd "$root/cmd"
	go run -mod=mod github.com/google/wire/cmd/wire
)
gofmt -w "$root"

echo
echo "scaffolded $root (http :$http_port, grpc :$grpc_port, db test_$app)"
echo "next steps:"
echo "  define your protos under api/<domain>/v1/, then: make api app=$app"
echo "  make build-$app && make run-$app"
