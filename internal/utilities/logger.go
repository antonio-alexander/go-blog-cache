package utilities

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Level int

const (
	Error Level = 1
	Info  Level = 2
	Debug Level = 3
	Trace Level = 4
)

func (l Level) String() string {
	switch l {
	default:
		return ""
	case Error:
		return "error"
	case Info:
		return "info"
	case Debug:
		return "debug"
	case Trace:
		return "trace"
	}
}

func AtoLogLevel(a string) Level {
	switch strings.ToLower(a) {
	default:
		return Error
	case "info":
		return Info
	case "debug":
		return Debug
	case "trace":
		return Trace
	}
}

type Logger interface {
	Configure(envs map[string]string) error
	Error(correlationId, format string, v ...any)
	Info(correlationId, format string, v ...any)
	Debug(correlationId, format string, v ...any)
	Trace(correlationId, format string, v ...any)
	Printf(format string, a ...any)
}

type logger struct {
	*log.Logger
	config struct {
		Prefix string `json:"prefix"`
		Level  Level  `json:"level"`
	}
}

func NewLogger(parameters ...any) Logger {
	return &logger{}
}

func (l *logger) Configure(envs map[string]string) error {
	if s, ok := envs["LOGGING_PREFIX"]; ok {
		l.config.Prefix = s
	}
	if s, ok := envs["LOGGING_LEVEL"]; ok {
		l.config.Level = AtoLogLevel(s)
	}
	if l.config.Prefix != "" {
		l.Logger = log.New(os.Stdout, fmt.Sprintf("[%s] ", l.config.Prefix),
			log.Ltime|log.Ldate|log.Lmsgprefix)
	} else {
		l.Logger = log.New(os.Stdout, "", log.Ltime|log.Ldate|log.Lmsgprefix)
	}
	return nil
}

func (l *logger) Error(correlationId, format string, v ...any) {
	if l.config.Level >= Error {
		l.Logger.Printf(correlationId+" [error] "+format, v...)
	}
}

func (l *logger) Info(correlationId, format string, v ...any) {
	if l.config.Level >= Info {
		l.Logger.Printf(correlationId+" [info] "+format, v...)
	}
}

func (l *logger) Debug(correlationId, format string, v ...any) {
	if l.config.Level >= Debug {
		l.Logger.Printf(correlationId+" [debug] "+format, v...)
	}
}

func (l *logger) Trace(correlationId, format string, v ...any) {
	if l.config.Level >= Trace {
		l.Logger.Printf(correlationId+" [trace] "+format, v...)
	}
}

func (l *logger) Printf(format string, v ...any) {
	if l.config.Level >= Trace {
		l.Logger.Printf(" [trace] "+format, v...)
	}
}
