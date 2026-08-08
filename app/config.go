package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

type Config struct {
	Logger  *slog.Logger
	Process string
}

func (conf Config) New(parent context.Context) (*App, error) {
	if conf.Process == "" {
		return nil, errors.New("empty process")
	}
	if conf.Logger == nil {
		conf.Logger = slog.Default()
	}
	proc, err := win.OpenProcess(conf.Process)
	if err != nil {
		return nil, fmt.Errorf("open process: %w", err)
	}
	app := &App{conf: conf}
	trackerConf := win.ProcessTrackerConfig{
		OnClose: []func(){func() {
			_ = app.Close(false)
		}},
		Process: proc,
	}
	app.tr, err = trackerConf.New()
	if err != nil {
		return nil, fmt.Errorf("create tracker: %w", err)
	}
	app.ctx, app.cancel = context.WithCancel(parent)
	return app, nil
}
