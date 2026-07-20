// Package log 是全项目统一日志的引导层,基于标准库 log/slog。
// 所有业务代码直接调用 slog.Debug/Info/Warn/Error;本包只负责在启动时
// 配置全局默认 logger 的输出格式与级别(通过 main 的 -log-level)。
package log

import (
	"log/slog"
	"os"
	"strings"
)

// levelVar 允许运行时调整全局日志级别。
var levelVar = new(slog.LevelVar)

// LevelSilent 高于 Error,用于 "silent" 静默所有日志。
const LevelSilent = slog.Level(1000)

func init() {
	// 在 Setup 之前也保证统一格式与合理默认级别(info)。
	levelVar.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: levelVar})))
}

// Setup 设置全局日志级别。level 取值:debug/info/warning/error/silent。
func Setup(level string) {
	levelVar.Set(ParseLevel(level))
}

// ParseLevel 把级别字符串映射为 slog.Level(未知值回退 info)。
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warning", "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "silent", "off", "none":
		return LevelSilent
	default:
		return slog.LevelInfo
	}
}
