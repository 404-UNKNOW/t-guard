package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

// Init 初始化生产级结构化日志
func Init() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	
	// 创建基础 Logger
	l, _ := config.Build()
	Log = l
}

// Mask 敏感字段脱敏处理
func Mask(key string, value string) zap.Field {
	k := strings.ToLower(key)
	if strings.Contains(k, "key") || strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "password") {
		return zap.String(key, "***")
	}
	return zap.String(key, value)
}

// Audit 审计日志专用
func Audit(event string, fields ...zap.Field) {
	Log.Info("[AUDIT] "+event, fields...)
}
