package server

import (
	"github.com/google/wire"
)

// ProviderSet 是 server 层的提供者集合。
var ProviderSet = wire.NewSet(NewGRPCServer, NewHTTPServer)
