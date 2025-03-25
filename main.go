package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"
	"syscall"

	"github.com/actions-oss/act-cli/cmd"
	"github.com/actions-oss/act-cli/pkg/common"
)

//go:embed VERSION
var version string

func main() {
	ctx := context.Background()
	ctx, forceCancel := context.WithCancel(ctx)
	cancelCtx, cancel := context.WithCancel(ctx)
	ctx = common.WithJobCancelContext(ctx, cancelCtx)

	// trap Ctrl+C and call cancel on the context
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
		cancel()
		forceCancel()
	}()
	go func() {
		select {
		case sig := <-c:
			if sig == os.Interrupt {
				cancel()
				select {
				case <-c:
					forceCancel()
				case <-ctx.Done():
				}
			} else {
				forceCancel()
			}
		case <-ctx.Done():
		}
	}()

	// run the command
	cmd.Execute(ctx, version)
}
