// Package version 提供 GoShell 的构建版本信息。
package version

// Version 是构建时通过 -ldflags 注入的正式版本号。
// 本地开发未注入版本时使用 dev。
var Version = "dev"
