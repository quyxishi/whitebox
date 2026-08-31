package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"path"
	"strings"
)

var levelVar = new(slog.LevelVar)

func ParseLevel(name string) (slog.Level, error) {
	var level slog.Level

	if err := level.UnmarshalText([]byte(strings.TrimSpace(name))); err != nil {
		return level, fmt.Errorf("unexpected log level: %s", name)
	}

	return level, nil
}

func SetLevel(level slog.Level) {
	levelVar.Set(level)
}

func Level() slog.Level {
	return levelVar.Level()
}

func Enabled(level slog.Level) bool {
	return level >= levelVar.Level()
}

type ModuleHandlerOptions struct {
	SlogOpts slog.HandlerOptions
}

type ModuleHandler struct {
	slog.Handler
	l *log.Logger
}

func (h *ModuleHandler) Handle(ctx context.Context, r slog.Record) error {
	var atr strings.Builder
	var mod string
	iat := r.Time.Format("2006-01-02 15:04:05.000000")

	r.Attrs(func(a slog.Attr) bool {
		if atr.Len() > 0 {
			atr.WriteByte(' ')
		}

		atr.WriteString(a.Key)
		atr.WriteByte(':')
		atr.WriteString(a.Value.String())
		return true
	})

	if src := r.Source(); src != nil && len(src.Function) > 0 {
		mod = strings.ReplaceAll(path.Base(src.Function), ".", "/") + ":"
	}

	h.l.Println(iat, "["+r.Level.String()+"]", mod, r.Message, atr.String())

	return nil
}

func NewModuleHandler(
	w io.Writer,
	opts *ModuleHandlerOptions,
) *ModuleHandler {
	if opts.SlogOpts.Level == nil {
		opts.SlogOpts.Level = levelVar
	}

	return &ModuleHandler{
		Handler: slog.NewTextHandler(w, &opts.SlogOpts),
		l:       log.New(w, "", 0),
	}
}
