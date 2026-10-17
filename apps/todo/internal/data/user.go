package data

import (
	"context"

	v1 "kratos-monorepo/api/user/v1"
	"kratos-monorepo/apps/todo/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"google.golang.org/grpc"
)

var (
	// ErrUserNotFound 在引用的用户不存在于 user 服务时返回。
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	// ErrUserUnavailable 在 user 服务不可达时返回。
	ErrUserUnavailable = errors.ServiceUnavailable(v1.ErrorReason_USER_UNAVAILABLE.String(), "user service unavailable")
)

// UserStore 是构建在 user 服务 gRPC 客户端之上的存储，service 层依赖这个
// 具体类型。其方法只使用 proto 类型：id 为 proto 的 `string` 形式。
type UserStore struct {
	client v1.UserServiceClient
}

// NewUserConn 打开一条到 user 服务的 gRPC 连接。目标地址来自配置：
// 本地开发用直连 host:port，接入注册中心后可换成 discovery:///user。
func NewUserConn(c *conf.Client) (*grpc.ClientConn, func(), error) {
	conn, err := kgrpc.NewClient(context.Background(),
		kgrpc.WithEndpoint(c.GetAddr()),
		kgrpc.WithTimeout(c.GetTimeout().AsDuration()),
	)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		conn.Close()
	}
	return conn, cleanup, nil
}

// NewUserStore 基于 user 服务的 gRPC 客户端创建一个新的 UserStore 实例。
func NewUserStore(conn *grpc.ClientConn) *UserStore {
	return &UserStore{client: v1.NewUserServiceClient(conn)}
}

func (s *UserStore) GetUser(ctx context.Context, id string) (*v1.User, error) {
	reply, err := s.client.GetUser(ctx, &v1.GetUserRequest{Id: id})
	if err != nil {
		// 把传输层错误映射为类型化错误，使上层调用方不必感知 RPC 层。
		if errors.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, ErrUserUnavailable
	}
	return reply, nil
}
