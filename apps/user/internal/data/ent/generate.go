package ent

// 按需给 schema 增加 --feature 参数，例如需要 OnConflict 支持时加
// sql/upsert，需要原生 SQL 时加 sql/execquery。
//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema
