package hotkey_test

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/something-that-is-cool/zutil/pkg/win/hotkey"
)

func ExampleManager() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mgr := hotkey.ManagerConfig{
		Handlers: map[string]func(){
			"h": func() {
				fmt.Println("h")
			},
			"s": func() {
				fmt.Println("s")
			},
		},
	}.New()

	err := mgr.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		panic(fmt.Errorf("error running hotkey manger: %w", err))
	}
}
