// Package logger is a thin wrapper over zap so the rest of the code does not
// import it directly.
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Field = zap.Field

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Sync() error
}

type zapLogger struct{ l *zap.Logger }

func New(level, namespace string) Logger {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	l, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		l = zap.NewNop()
	}

	return &zapLogger{l: l.Named(namespace)}
}

func (z *zapLogger) Debug(msg string, f ...Field) { z.l.Debug(msg, f...) }
func (z *zapLogger) Info(msg string, f ...Field)  { z.l.Info(msg, f...) }
func (z *zapLogger) Warn(msg string, f ...Field)  { z.l.Warn(msg, f...) }
func (z *zapLogger) Error(msg string, f ...Field) { z.l.Error(msg, f...) }
func (z *zapLogger) Sync() error                  { return z.l.Sync() }

func String(key, value string) Field  { return zap.String(key, value) }
func Int(key string, value int) Field { return zap.Int(key, value) }
func Error(err error) Field           { return zap.Error(err) }

// Cleanup flushes buffered entries. A failing Sync on a terminal is harmless.
func Cleanup(l Logger) error { return l.Sync() }
