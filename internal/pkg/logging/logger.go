package logging

import "github.com/Aliizi83/vohu/internal/config"

type Logger interface {
	Init()

	Debug(category Category, subCategory SubCategory, message string, extra map[string]any)
	Debugf(template string, args ...any)

	Info(category Category, subCategory SubCategory, message string, extra map[string]any)
	Infof(template string, args ...any)

	Warning(category Category, subCategory SubCategory, message string, extra map[string]any)
	Warningf(template string, args ...any)

	Error(err error, category Category, subCategory SubCategory, message string, extra map[string]any)
	Errorf(err error, template string, args ...any)

	Fatal(err error, category Category, subCategory SubCategory, message string, extra map[string]any)
	Fatalf(err error, template string, args ...any)
}

func NewLogger(cfg *config.Config) Logger {
	switch cfg.Logger.Logger {
	case "zap":
		return NewZapLogger(cfg)
	case "zero":
		return NewZeroLogger(cfg)
	default:
		return NewZapLogger(cfg)
	}
}
