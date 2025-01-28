//go:build !darwin

package runner

import (
	"context"
	"fmt"

	"github.com/actions-oss/act-cli/pkg/common"
)

func (rc *RunContext) startTartEnvironment() common.Executor {
	return func(_ context.Context) error {
		return fmt.Errorf("You need macOS for tart")
	}
}
