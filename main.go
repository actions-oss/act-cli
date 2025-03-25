package main

import (
	_ "embed"

	"github.com/actions-oss/act-cli/cmd"
	"github.com/actions-oss/act-cli/pkg/common"
)

//go:embed VERSION
var version string

func main() {
	ctx, cancel := common.CreateGracefulJobCancellationContext()
	defer cancel()

	// run the command
	cmd.Execute(ctx, version)
}
