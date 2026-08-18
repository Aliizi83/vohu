package logging

import (
	"sync"

	"github.com/Aliizi83/vohu/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var zapLogLevelMap = map[string]zapcore.Level{
	"debug":   zapcore.DebugLevel,
	"info":    zapcore.InfoLevel,
	"warning": zapcore.WarnLevel,
	"error":   zapcore.ErrorLevel,
	"fatal":   zapcore.FatalLevel,
}

var once sync.Once
var zapSinLogger *zap.SugaredLogger

type ZapLogger struct {
	cfg    *config.Config
	logger *zap.SugaredLogger
}

func NewZapLogger(cfg *config.Config) *ZapLogger {
	logger := &ZapLogger{cfg: cfg}
	logger.Init()
	return logger
}

func (z *ZapLogger) getLogLevel() zapcore.Level {
	level, ok := zapLogLevelMap[z.cfg.Logger.Level]
	if !ok {
		return zapcore.InfoLevel
	}
	return level
}

func (z *ZapLogger) Init() {
	once.Do(func() {
		w := zapcore.AddSync(&lumberjack.Logger{
			Filename:   GetLogFileNamePerDay(z.cfg.Logger.FileFolderPath),
			MaxSize:    z.cfg.Logger.MaxLogSize,
			MaxAge:     int(z.cfg.Logger.MaxLogAge),
			Compress:   true,
			LocalTime:  true,
			MaxBackups: 10,
		})

		config := zap.NewProductionEncoderConfig()
		config.EncodeTime = zapcore.ISO8601TimeEncoder

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(config),
			w,
			z.getLogLevel(),
		)
		zapSinLogger = zap.New(core, zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel)).Sugar()
	})

	z.logger = zapSinLogger
}

func (z *ZapLogger) Debug(category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	pairs := getPairs(extra, category, subCategory)
	z.logger.Debugw(message, pairs...)
}

func (z *ZapLogger) Debugf(template string, args ...interface{}) {
	z.logger.Debugf(template, args...)
}

func (z *ZapLogger) Info(category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	pairs := getPairs(extra, category, subCategory)
	z.logger.Infow(message, pairs...)
}
func (z *ZapLogger) Infof(template string, args ...interface{}) {
	z.logger.Infof(template, args...)
}

func (z *ZapLogger) Warning(category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	pairs := getPairs(extra, category, subCategory)
	z.logger.Warnw(message, pairs...)
}
func (z *ZapLogger) Warningf(template string, args ...interface{}) {
	z.logger.Warnf(template, args...)
}

func (z *ZapLogger) Error(err error, category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	pairs := getPairs(extra, category, subCategory)
	z.logger.Errorw(message, append(pairs, "error", err)...)
}
func (z *ZapLogger) Errorf(err error, template string, args ...interface{}) {
	z.logger.Errorf(template, args...)
}

func (z *ZapLogger) Fatal(err error, category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	pairs := getPairs(extra, category, subCategory)
	z.logger.Fatalw(message, append(pairs, "error", err)...)
}
func (z *ZapLogger) Fatalf(err error, template string, args ...interface{}) {
	z.logger.Fatalf(template, args...)
}

func getPairs(extra map[string]interface{}, category Category, subCategory SubCategory) []interface{} {
	if extra == nil {
		extra = make(map[string]interface{})
	}
	extra["Category"] = category
	extra["SubCategory"] = subCategory
	pairs := ConvertMapToInterface(extra)
	return pairs
}
