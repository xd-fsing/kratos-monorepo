package service

import "github.com/google/wire"

// ProviderSet 是 service 层的提供者集合。
var ProviderSet = wire.NewSet(NewTodoService)
