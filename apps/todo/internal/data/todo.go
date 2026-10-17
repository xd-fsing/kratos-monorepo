package data

import (
	"context"

	v1 "kratos-monorepo/api/todo/v1"
	"kratos-monorepo/apps/todo/internal/data/ent"
	"kratos-monorepo/apps/todo/internal/data/ent/todo"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	// ErrTodoNotFound 在 todo 不存在时返回。
	ErrTodoNotFound = errors.NotFound(v1.ErrorReason_TODO_NOT_FOUND.String(), "todo not found")
	// ErrTodoInvalidArgument 在 todo 请求非法时返回。
	ErrTodoInvalidArgument = errors.BadRequest(v1.ErrorReason_TODO_INVALID_ARGUMENT.String(), "invalid todo argument")
)

// TodoStore 是构建在 ent 之上的 todo 存储，service 层依赖这个具体类型。
// 其方法只使用 proto 类型：id 为 proto 的 `string` 形式，分页大小对应
// proto 的 `int32`。
type TodoStore struct {
	data *Data
}

// NewTodoStore 创建一个新的 TodoStore 实例。
func NewTodoStore(data *Data) *TodoStore {
	return &TodoStore{data: data}
}

func (s *TodoStore) FindByID(ctx context.Context, id string) (*v1.Todo, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		// service 层已校验格式；此处是防御性检查，避免非法值触达驱动层。
		return nil, ErrTodoInvalidArgument
	}
	po, err := s.data.db.Todo.Query().
		Where(todo.IDEQ(parsed), todo.StatusEQ(int32(v1.TodoStatus_TODO_STATUS_ACTIVE))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrTodoNotFound
		}
		return nil, err
	}
	// 持久化结构与 api 结构不同（id 是 uuid 而非 string、status 是 int32
	// 而非枚举、时间是 time.Time 而非 Timestamp），因此读取时在返回处
	// 内联转换。
	reply := &v1.Todo{
		Id:        po.ID.String(),
		Title:     po.Title,
		Content:   po.Content,
		Completed: po.Completed,
		Status:    v1.TodoStatus(po.Status),
		CreatedAt: timestamppb.New(po.CreatedAt),
		UpdatedAt: timestamppb.New(po.UpdatedAt),
	}
	if po.UserID != nil {
		reply.UserId = po.UserID.String()
	}
	return reply, nil
}

func (s *TodoStore) ListTodos(ctx context.Context, filter filtering.Filter, orderBy ordering.OrderBy, offset, limit int32) ([]*v1.Todo, error) {
	if offset < 0 || limit <= 0 {
		return nil, ErrTodoInvalidArgument
	}
	// 偏移分页需要全序，因此 id 总是作为最后一个排序键追加：否则在同一
	// 字段上并列的行可能每次查询返回顺序不同，导致跨页重复或漏行。
	// UUIDv7 的 id 按时间有序，因此调用方未指定排序时，单独按 id 排序
	// 就是合理的默认值。
	pos, err := s.data.db.Todo.Query().
		Where(todo.StatusEQ(int32(v1.TodoStatus_TODO_STATUS_ACTIVE))).
		Where(ents.ApplyFilter(filter)).
		Order(ents.ApplyOrderBy(orderBy), todo.ByID()).
		Offset(int(offset)).
		Limit(int(limit)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	todos := make([]*v1.Todo, 0, len(pos))
	for _, po := range pos {
		todo := &v1.Todo{
			Id:        po.ID.String(),
			Title:     po.Title,
			Content:   po.Content,
			Completed: po.Completed,
			Status:    v1.TodoStatus(po.Status),
			CreatedAt: timestamppb.New(po.CreatedAt),
			UpdatedAt: timestamppb.New(po.UpdatedAt),
		}
		if po.UserID != nil {
			todo.UserId = po.UserID.String()
		}
		todos = append(todos, todo)
	}
	return todos, nil
}

func (s *TodoStore) CreateTodo(ctx context.Context, t *v1.Todo) (*v1.Todo, error) {
	// user_id 只在创建时写入，更新路径不会触碰它。nil 指针落库为 NULL。
	// service 层已校验格式；此处是防御性检查，避免非法值触达驱动层。
	var userID *uuid.UUID
	if t.GetUserId() != "" {
		parsed, err := uuid.Parse(t.GetUserId())
		if err != nil {
			return nil, ErrTodoInvalidArgument
		}
		userID = &parsed
	}
	po, err := s.data.db.Todo.Create().
		SetTitle(t.GetTitle()).
		SetContent(t.GetContent()).
		SetCompleted(t.GetCompleted()).
		SetStatus(int32(v1.TodoStatus_TODO_STATUS_ACTIVE)).
		SetNillableUserID(userID).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.Todo{
		Id:        po.ID.String(),
		Title:     po.Title,
		Content:   po.Content,
		Completed: po.Completed,
		Status:    v1.TodoStatus(po.Status),
		CreatedAt: timestamppb.New(po.CreatedAt),
		UpdatedAt: timestamppb.New(po.UpdatedAt),
	}
	if po.UserID != nil {
		reply.UserId = po.UserID.String()
	}
	return reply, nil
}

func (s *TodoStore) UpdateTodo(ctx context.Context, t *v1.Todo) (*v1.Todo, error) {
	id, err := uuid.Parse(t.GetId())
	if err != nil {
		return nil, ErrTodoInvalidArgument
	}
	// status 与 user_id 在此不可更新：status 只能通过 DeleteTodo 流转，
	// user_id 在创建时固定。
	po, err := s.data.db.Todo.UpdateOneID(id).
		Where(todo.StatusEQ(int32(v1.TodoStatus_TODO_STATUS_ACTIVE))).
		SetTitle(t.GetTitle()).
		SetContent(t.GetContent()).
		SetCompleted(t.GetCompleted()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrTodoNotFound
		}
		return nil, err
	}
	reply := &v1.Todo{
		Id:        po.ID.String(),
		Title:     po.Title,
		Content:   po.Content,
		Completed: po.Completed,
		Status:    v1.TodoStatus(po.Status),
		CreatedAt: timestamppb.New(po.CreatedAt),
		UpdatedAt: timestamppb.New(po.UpdatedAt),
	}
	if po.UserID != nil {
		reply.UserId = po.UserID.String()
	}
	return reply, nil
}

// DeleteTodo 通过把 status 翻转为 deleted 来软删除 todo。记录保留以便
// 审计，所有读取路径都会过滤掉它。
func (s *TodoStore) DeleteTodo(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		// service 层已校验格式；此处是防御性检查，避免非法值触达驱动层。
		return ErrTodoInvalidArgument
	}
	affected, err := s.data.db.Todo.Update().
		Where(todo.IDEQ(parsed), todo.StatusEQ(int32(v1.TodoStatus_TODO_STATUS_ACTIVE))).
		SetStatus(int32(v1.TodoStatus_TODO_STATUS_DELETED)).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTodoNotFound
	}
	return nil
}
