package utils

import "github.com/rs/zerolog"

type LogWrapperInterface interface {
	Printf(format string, v ...interface{})
	Debug(s ...interface{})
}

type logWrapper struct {
	logger zerolog.Logger
}

func (l logWrapper) Printf(format string, v ...interface{}) {
	l.logger.Info().Msgf(format, v...)
}

func (l logWrapper) Debug(args ...interface{}) {
	l.logger.Debug().Msgf("%v", args)
}

func WrapLogger(logger zerolog.Logger) LogWrapperInterface {
	return &logWrapper{logger: logger}
}
