package logging

import (
	"os"

	"github.com/Aliizi83/vohu/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var zeroSinLogger *zerolog.Logger

var zeroLogLevelMap = map[string]zerolog.Level{
	"debug":   zerolog.DebugLevel,
	"info":    zerolog.InfoLevel,
	"warning": zerolog.WarnLevel,
	"error":   zerolog.ErrorLevel,
	"fatal":   zerolog.FatalLevel,
}

type ZeroLogger struct {
	cfg    *config.Config
	logger *zerolog.Logger
}

func NewZeroLogger(cfg *config.Config) *ZeroLogger {
	logger := &ZeroLogger{cfg: cfg}
	logger.Init()
	return logger
}

func (z *ZeroLogger) getLogLevel() zerolog.Level {
	level, ok := zeroLogLevelMap[z.cfg.Logger.Level]
	if !ok {
		return zerolog.InfoLevel
	}
	return level
}

func (z *ZeroLogger) Init() {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		fileName := GetLogFileNamePerDay(z.cfg.Logger.FileFolderPath)

		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			panic("could not open log file while initializing zero logger: " + err.Error())
		}

		var logger = zerolog.New(file).With().Timestamp().Str("AppName", z.cfg.Server.AppName).Str("Logger", "ZeroLog").Logger()
		zerolog.SetGlobalLevel(z.getLogLevel())
		zeroSinLogger = &logger
	})

	z.logger = zeroSinLogger
}

func (z *ZeroLogger) Debug(category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	z.logger.
		Debug().
		Str("Category", string(category)).
		Str("SubCategory", string(subCategory)).
		Fields(extra).
		Msg(message)
}
func (z *ZeroLogger) Debugf(template string, args ...interface{}) {
	z.logger.
		Debug().
		Msgf(template, args...)
}

func (z *ZeroLogger) Info(category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	z.logger.
		Info().
		Str("Category", string(category)).
		Str("SubCategory", string(subCategory)).
		Fields(extra).
		Msg(message)
}
func (z *ZeroLogger) Infof(template string, args ...interface{}) {
	z.logger.
		Info().
		Msgf(template, args...)
}

func (z *ZeroLogger) Warning(category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	z.logger.
		Warn().
		Str("Category", string(category)).
		Str("SubCategory", string(subCategory)).
		Fields(extra).
		Msg(message)
}
func (z *ZeroLogger) Warningf(template string, args ...interface{}) {
	z.logger.
		Warn().
		Msgf(template, args...)
}

func (z *ZeroLogger) Error(err error, category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	z.logger.
		Error().
		Str("Category", string(category)).
		Str("SubCategory", string(subCategory)).
		Fields(extra).
		Err(err).
		Msg(message)
}
func (z *ZeroLogger) Errorf(err error, template string, args ...interface{}) {
	z.logger.
		Error().
		Err(err).
		Msgf(template, args...)
}

func (z *ZeroLogger) Fatal(err error, category Category, subCategory SubCategory, message string, extra map[string]interface{}) {
	z.logger.
		Fatal().
		Str("Category", string(category)).
		Str("SubCategory", string(subCategory)).
		Fields(extra).
		Err(err).
		Msg(message)
}
func (z *ZeroLogger) Fatalf(err error, template string, args ...interface{}) {
	z.logger.
		Fatal().
		Err(err).
		Msgf(template, args...)
}
