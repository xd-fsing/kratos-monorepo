package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	v1 "kratos-monorepo/api/todo/v1"
)

// Todo 保存 Todo 实体的 schema 定义。
type Todo struct {
	ent.Schema
}

// Mixin of the Todo.
func (Todo) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Todo.
func (Todo) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").Default(""),
		field.String("content").Default(""),
		field.Bool("completed").Default(false),
		// status 标记记录的生命周期：删除只是把它翻转为 deleted 而不是
		// 移除记录，因此所有读取都按 active 过滤。以普通 int32 存储，取值
		// 与 api 枚举一致，由 data 层负责转换。
		field.Int32("status").
			Default(int32(v1.TodoStatus_TODO_STATUS_ACTIVE)),
		// user_id 引用由 user 服务持有的用户。不建外键：该记录存在于
		// 另一个服务的数据库里，存在性通过那个服务的 API 校验。
		field.UUID("user_id", uuid.Nil).Optional().Nillable(),
	}
}

// Indexes of the Todo.
func (Todo) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
	}
}
