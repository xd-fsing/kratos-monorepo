package service

import (
	"context"
	"errors"
	"io"
	"strings"

	v1 "kratos-monorepo/api/todo/v1"
	"kratos-monorepo/apps/todo/internal/data"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageSize = 20
)

// TodoService 是 todo 服务。它在边界处校验请求，并编排 data.TodoStore 与
// user 服务的 data.UserStore；所有业务规则都在这里，存储位于各 store 之后。
type TodoService struct {
	v1.UnimplementedTodoServiceServer

	store *data.TodoStore
	users *data.UserStore
}

// NewTodoService 创建一个新的 todo 服务。
func NewTodoService(store *data.TodoStore, users *data.UserStore) *TodoService {
	return &TodoService{store: store, users: users}
}

// CreateTodo 创建一条 todo。
func (s *TodoService) CreateTodo(ctx context.Context, req *v1.CreateTodoRequest) (*v1.Todo, error) {
	todo := req.GetTodo()
	if err := validateTodo(todo); err != nil {
		return nil, err
	}
	if todo.GetUserId() != "" {
		if err := validateID(todo.GetUserId()); err != nil {
			return nil, err
		}
		// 归属用户的数据在 user 服务的数据库里，因此存在性要通过它的 API
		// 校验，而不是通过外键。
		if _, err := s.users.GetUser(ctx, todo.GetUserId()); err != nil {
			return nil, err
		}
	}
	return s.store.CreateTodo(ctx, todo)
}

// GetTodo 按 ID 返回一条 todo。
func (s *TodoService) GetTodo(ctx context.Context, req *v1.GetTodoRequest) (*v1.Todo, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	return s.store.FindByID(ctx, req.GetId())
}

// ListTodos 列出 todo。
func (s *TodoService) ListTodos(ctx context.Context, req *v1.ListTodosRequest) (*v1.TodoSet, error) {
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("id", filtering.TypeString),
		filtering.DeclareIdent("title", filtering.TypeString),
		filtering.DeclareIdent("content", filtering.TypeString),
		filtering.DeclareIdent("completed", filtering.TypeBool),
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
	if err := orderBy.ValidateForPaths("id", "title", "created_at", "updated_at"); err != nil {
		return nil, err
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	todos, err := s.store.ListTodos(ctx, filter, orderBy, int32(pageToken.Offset), req.PageSize)
	if err != nil {
		return nil, err
	}
	set := &v1.TodoSet{
		Todos: make([]*v1.Todo, 0, len(todos)),
	}
	if len(todos) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, todo := range todos {
		set.Todos = append(set.Todos, todo)
	}
	return set, nil
}

// UpdateTodo 更新一条 todo。
func (s *TodoService) UpdateTodo(ctx context.Context, req *v1.UpdateTodoRequest) (*v1.Todo, error) {
	if req.GetTodo().GetId() == "" || req.GetUpdateMask() == nil || len(req.GetUpdateMask().GetPaths()) == 0 {
		return nil, data.ErrTodoInvalidArgument
	}
	current, err := s.GetTodo(ctx, &v1.GetTodoRequest{Id: req.GetTodo().GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.GetUpdateMask(), current, req.GetTodo())
	return s.store.UpdateTodo(ctx, current)
}

// DeleteTodo 软删除一条 todo。
func (s *TodoService) DeleteTodo(ctx context.Context, req *v1.DeleteTodoRequest) (*emptypb.Empty, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	if err := s.store.DeleteTodo(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// WatchTodos 把 todo 快照从服务端推送给客户端。
func (s *TodoService) WatchTodos(req *v1.WatchTodosRequest, stream v1.TodoService_WatchTodosServer) error {
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("id", filtering.TypeString),
		filtering.DeclareIdent("title", filtering.TypeString),
		filtering.DeclareIdent("content", filtering.TypeString),
		filtering.DeclareIdent("completed", filtering.TypeBool),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
		filtering.DeclareIdent("updated_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return err
	}
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return err
	}
	if err := orderBy.ValidateForPaths("id", "title", "created_at", "updated_at"); err != nil {
		return err
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	todos, err := s.store.ListTodos(stream.Context(), filter, orderBy, int32(pageToken.Offset), req.PageSize)
	if err != nil {
		return err
	}
	for _, todo := range todos {
		if err := stream.Send(newTodoEvent("snapshot", todo)); err != nil {
			return err
		}
	}
	return nil
}

// SyncTodos 双向交换 todo 变更。
func (s *TodoService) SyncTodos(stream v1.TodoService_SyncTodosServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var event *v1.TodoEvent
		switch strings.ToLower(req.GetAction()) {
		case "create":
			todo, err := s.CreateTodo(stream.Context(), &v1.CreateTodoRequest{Todo: req.GetTodo()})
			if err != nil {
				return err
			}
			event = newTodoEvent("created", todo)
		case "update":
			todo, err := s.UpdateTodo(stream.Context(), &v1.UpdateTodoRequest{
				Todo:       req.GetTodo(),
				UpdateMask: req.GetUpdateMask(),
			})
			if err != nil {
				return err
			}
			event = newTodoEvent("updated", todo)
		case "delete":
			id := req.GetId()
			if id == "" {
				id = req.GetTodo().GetId()
			}
			if _, err := s.DeleteTodo(stream.Context(), &v1.DeleteTodoRequest{Id: id}); err != nil {
				return err
			}
			// 记录此后对读取不可见，因此只有 id 与结果 status 有意义。
			event = newTodoEvent("deleted", &v1.Todo{
				Id:     id,
				Status: v1.TodoStatus_TODO_STATUS_DELETED,
			})
		default:
			return data.ErrTodoInvalidArgument
		}
		if err := stream.Send(event); err != nil {
			return err
		}
	}
}

// validateID 在服务边界校验传入的 id 是否为 UUID。流向各 store 的是原始
// string，因此这里的校验只把关格式。
func validateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return data.ErrTodoInvalidArgument
	}
	return nil
}

// validateTodo 在交给 store 之前校验传入 todo 的可变字段。
func validateTodo(todo *v1.Todo) error {
	if todo == nil || strings.TrimSpace(todo.GetTitle()) == "" {
		return data.ErrTodoInvalidArgument
	}
	return nil
}

// newTodoEvent 把对外返回的 todo 包进一个流事件。
func newTodoEvent(action string, todo *v1.Todo) *v1.TodoEvent {
	return &v1.TodoEvent{
		Action:    action,
		Todo:      todo,
		EventTime: timestamppb.Now(),
	}
}
