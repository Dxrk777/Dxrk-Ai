// SPDX-License-Identifier: MIT
package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZap(level Level) (Logger, error) {
	zapLevel := zapcore.InfoLevel
	switch level {
	case LevelDebug:
		zapLevel = zapcore.DebugLevel
	case LevelInfo:
		zapLevel = zapcore.InfoLevel
	case LevelWarn:
		zapLevel = zapcore.WarnLevel
	case LevelError:
		zapLevel = zapcore.ErrorLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	z, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return &zapAdapter{logger: z, level: level}, nil
}

type zapAdapter struct {
	logger *zap.Logger
	level  Level
}

func (a *zapAdapter) Debug(msg string, args ...any) { a.logger.Sugar().Debugw(msg, args...) }
func (a *zapAdapter) Info(msg string, args ...any)  { a.logger.Sugar().Infow(msg, args...) }
func (a *zapAdapter) Warn(msg string, args ...any)  { a.logger.Sugar().Warnw(msg, args...) }
func (a *zapAdapter) Error(msg string, args ...any) { a.logger.Sugar().Errorw(msg, args...) }
func (a *zapAdapter) With(args ...any) Logger {
	return &zapAdapter{logger: a.logger.With(convertArgsToFields(args)...), level: a.level}
}
func (a *zapAdapter) Level() Level { return a.level }

func convertArgsToFields(args []any) []zap.Field {
	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args)-1; i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, args[i+1]))
	}
	return fields
}

func NewZapNop() Logger {
	return &zapAdapter{logger: zap.NewNop(), level: LevelInfo}
}
