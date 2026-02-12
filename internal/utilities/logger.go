package utilities

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/antonio-alexander/go-blog-cache/internal"
)

type logger struct {
	*log.Logger
	config struct {
		Level Level
	}
}

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

type Logger interface {
	Error(ctx context.Context, format string, v ...any)
	Info(ctx context.Context, format string, v ...any)
	Debug(ctx context.Context, format string, v ...any)
	Trace(ctx context.Context, format string, v ...any)
}

func atoLogLevel(a string) Level {
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

func NewLogger() interface {
	internal.Configurer
	Logger
} {
	return &logger{
		Logger: log.New(os.Stdout, "", log.Ltime|log.Ldate|log.Lmsgprefix),
	}
}

func (l *logger) Configure(envs map[string]string) error {
	l.config.Level = Error
	if logLevel, ok := envs["LOG_LEVEL"]; ok {
		l.config.Level = atoLogLevel(logLevel)
	}
	return nil
}

func (l *logger) Error(ctx context.Context, format string, v ...any) {
	if l.config.Level < Error {
		return
	}
	prefix := "[error] "
	if correlationId := internal.CorrelationIdFromCtx(ctx); correlationId != "" {
		prefix = fmt.Sprintf("[error] (%s) ", correlationId)
	}
	l.Printf(prefix+format, v...)
}

func (l *logger) Info(ctx context.Context, format string, v ...any) {
	if l.config.Level < Info {
		return
	}
	prefix := "[info] "
	if correlationId := internal.CorrelationIdFromCtx(ctx); correlationId != "" {
		prefix = fmt.Sprintf("[info] (%s) ", correlationId)
	}
	l.Printf(prefix+format, v...)
}

func (l *logger) Debug(ctx context.Context, format string, v ...any) {
	if l.config.Level < Debug {
		return
	}
	prefix := "[debug] "
	if correlationId := internal.CorrelationIdFromCtx(ctx); correlationId != "" {
		prefix = fmt.Sprintf("[debug] (%s) ", correlationId)
	}
	l.Printf(prefix+format, v...)
}

func (l *logger) Trace(ctx context.Context, format string, v ...any) {
	if l.config.Level < Trace {
		return
	}
	prefix := "[trace] "
	if correlationId := internal.CorrelationIdFromCtx(ctx); correlationId != "" {
		prefix = fmt.Sprintf("[trace] (%s) ", correlationId)
	}
	l.Printf(prefix+format, v...)
}
