package service

import (
	"context"
	"strings"

	v1 "kratos-monorepo/api/user/v1"
	"kratos-monorepo/apps/user/internal/data"

	"github.com/google/uuid"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
)

const (
	defaultPageSize = 20
)

// UserService 是用户服务。它在边界处校验请求，并把工作委托给
// data.UserStore；所有业务规则都在这里，存储位于 store 之后。
type UserService struct {
	v1.UnimplementedUserServiceServer

	store *data.UserStore
}

// NewUserService 创建一个新的用户服务。
func NewUserService(store *data.UserStore) *UserService {
	return &UserService{store: store}
}

// CreateUser 创建一个用户。
func (s *UserService) CreateUser(ctx context.Context, req *v1.CreateUserRequest) (*v1.User, error) {
	user := req.GetUser()
	if err := validateUser(user); err != nil {
		return nil, err
	}
	return s.store.CreateUser(ctx, user)
}

// GetUser 按 ID 返回一个用户。
func (s *UserService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.User, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	return s.store.FindByID(ctx, req.GetId())
}

// ListUsers 列出用户。
func (s *UserService) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.UserSet, error) {
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("id", filtering.TypeString),
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("email", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
		filtering.DeclareIdent("updated_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	if err := orderBy.ValidateForPaths("id", "name", "email", "created_at", "updated_at"); err != nil {
		return nil, err
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	users, err := s.store.ListUsers(ctx, filter, orderBy, int32(pageToken.Offset), req.PageSize)
	if err != nil {
		return nil, err
	}
	set := &v1.UserSet{
		Users: make([]*v1.User, 0, len(users)),
	}
	if len(users) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, user := range users {
		set.Users = append(set.Users, user)
	}
	return set, nil
}

// validateID 在服务边界校验传入的 id 是否为 UUID。流向 store 的是原始
// string，因此这里的校验只把关格式。
func validateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return data.ErrUserInvalidArgument
	}
	return nil
}

// validateUser 在交给 store 之前校验传入用户的可变字段。
func validateUser(user *v1.User) error {
	if user == nil || strings.TrimSpace(user.GetName()) == "" || strings.TrimSpace(user.GetEmail()) == "" {
		return data.ErrUserInvalidArgument
	}
	return nil
}
