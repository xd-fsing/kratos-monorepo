package data

import (
	"context"

	"kratos-monorepo/apps/user/internal/conf"
	"kratos-monorepo/apps/user/internal/data/ent"

	"github.com/go-kratos/kratos/v3/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
)

// ProviderSet 是 data 层的提供者集合。
var ProviderSet = wire.NewSet(NewData, NewUserStore)

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
