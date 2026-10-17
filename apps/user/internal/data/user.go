package data

import (
	"context"

	v1 "kratos-monorepo/api/user/v1"
	"kratos-monorepo/apps/user/internal/data/ent"
	"kratos-monorepo/apps/user/internal/data/ent/user"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	// ErrUserNotFound 在用户不存在时返回。
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	// ErrUserInvalidArgument 在用户请求非法时返回。
	ErrUserInvalidArgument = errors.BadRequest(v1.ErrorReason_USER_INVALID_ARGUMENT.String(), "invalid user argument")
)

// UserStore 是构建在 ent 之上的用户存储，service 层依赖这个具体类型。
// 其方法只使用 proto 类型：id 为 proto 的 `string` 形式，分页大小对应
// proto 的 `int32`。
type UserStore struct {
	data *Data
}

// NewUserStore 创建一个新的 UserStore 实例。
func NewUserStore(data *Data) *UserStore {
	return &UserStore{data: data}
}

func (s *UserStore) FindByID(ctx context.Context, id string) (*v1.User, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		// service 层已校验格式；此处是防御性检查，避免非法值触达驱动层。
		return nil, ErrUserInvalidArgument
	}
	po, err := s.data.db.User.Query().
		Where(user.IDEQ(parsed)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	// 持久化结构与 api 结构不同（id 是 uuid 而非 string、时间是 time.Time
	// 而非 Timestamp），因此读取时在返回处内联转换。
	return &v1.User{
		Id:        po.ID.String(),
		Name:      po.Name,
		Email:     po.Email,
		CreatedAt: timestamppb.New(po.CreatedAt),
		UpdatedAt: timestamppb.New(po.UpdatedAt),
	}, nil
}

func (s *UserStore) ListUsers(ctx context.Context, filter filtering.Filter, orderBy ordering.OrderBy, offset, limit int32) ([]*v1.User, error) {
	if offset < 0 || limit <= 0 {
		return nil, ErrUserInvalidArgument
	}
	// 偏移分页需要全序，因此 id 总是作为最后一个排序键追加：否则在同一
	// 字段上并列的行可能每次查询返回顺序不同，导致跨页重复或漏行。
	// UUIDv7 的 id 按时间有序，因此调用方未指定排序时，单独按 id 排序
	// 就是合理的默认值。
	pos, err := s.data.db.User.Query().
		Where(ents.ApplyFilter(filter)).
		Order(ents.ApplyOrderBy(orderBy), user.ByID()).
		Offset(int(offset)).
		Limit(int(limit)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]*v1.User, 0, len(pos))
	for _, po := range pos {
		users = append(users, &v1.User{
			Id:        po.ID.String(),
			Name:      po.Name,
			Email:     po.Email,
			CreatedAt: timestamppb.New(po.CreatedAt),
			UpdatedAt: timestamppb.New(po.UpdatedAt),
		})
	}
	return users, nil
}

func (s *UserStore) CreateUser(ctx context.Context, u *v1.User) (*v1.User, error) {
	po, err := s.data.db.User.Create().
		SetName(u.GetName()).
		SetEmail(u.GetEmail()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.User{
		Id:        po.ID.String(),
		Name:      po.Name,
		Email:     po.Email,
		CreatedAt: timestamppb.New(po.CreatedAt),
		UpdatedAt: timestamppb.New(po.UpdatedAt),
	}, nil
}
