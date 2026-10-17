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
