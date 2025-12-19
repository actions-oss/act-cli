package runner

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/actions-oss/act-cli/pkg/common"
	"github.com/actions-oss/act-cli/pkg/model"
)

type stepActionLocal struct {
	Step                *model.Step
	RunContext          *RunContext
	compositeRunContext *RunContext
	compositeSteps      *compositeSteps
	runAction           runAction
	readAction          readAction
	env                 map[string]string
	action              *model.Action
}

func (sal *stepActionLocal) pre() common.Executor {
	sal.env = map[string]string{}

	return func(_ context.Context) error {
		return nil
	}
}

func (sal *stepActionLocal) main() common.Executor {
	return runStepExecutor(sal, stepStageMain, func(ctx context.Context) error {
		if common.Dryrun(ctx) {
			return nil
		}

		workdir := sal.getRunContext().Config.Workdir
		actionDir := filepath.Join(workdir, sal.Step.Uses)

		localReader := func(ctx context.Context) actionYamlReader {
			// In case we want to limit resolving symlinks, folders are resolved by archive function
			// _, cpath = sal.getContainerActionPathsExt(".")
			roots := []string{
				".", // Allow everything, other code permits it already
				// path.Dir(cpath),                          // Allow RUNNER_WORKSPACE e.g. GITHUB_WORKSPACE/../
				// sal.RunContext.JobContainer.GetActPath(), // Allow remote action folders
			}
			_, cpath := sal.getContainerActionPaths()
			return func(filename string) (io.Reader, io.Closer, error) {
				spath := path.Join(cpath, filename)
				for i := 0; i < maxSymlinkDepth; i++ {
					tars, err := sal.RunContext.JobContainer.GetContainerArchive(ctx, spath)
					if errors.Is(err, fs.ErrNotExist) {
						return nil, nil, err
					} else if err != nil {
						return nil, nil, fs.ErrNotExist
					}
					treader := tar.NewReader(tars)
					header, err := treader.Next()
					if errors.Is(err, io.EOF) {
						return nil, nil, os.ErrNotExist
					} else if err != nil {
						return nil, nil, err
					}
					if header.FileInfo().Mode()&os.ModeSymlink == os.ModeSymlink {
						spath, err = symlinkJoin(spath, header.Linkname, roots...)
						if err != nil {
							return nil, nil, err
						}
					} else {
						return treader, tars, nil
					}
				}
				return nil, nil, fmt.Errorf("max depth %d of symlinks exceeded while reading %s", maxSymlinkDepth, spath)
			}
		}

		actionModel, err := sal.readAction(ctx, sal.Step, actionDir, "", localReader(ctx), os.WriteFile)
		if err != nil {
			return err
		}

		sal.action = actionModel

		return sal.runAction(sal)(ctx)
	})
}

func (sal *stepActionLocal) post() common.Executor {
	return runStepExecutor(sal, stepStagePost, runPostStep(sal)).If(hasPostStep(sal)).If(shouldRunPostStep(sal))
}

func (sal *stepActionLocal) getRunContext() *RunContext {
	return sal.RunContext
}

func (sal *stepActionLocal) getGithubContext(ctx context.Context) *model.GithubContext {
	return sal.getRunContext().getGithubContext(ctx)
}

func (sal *stepActionLocal) getStepModel() *model.Step {
	return sal.Step
}

func (sal *stepActionLocal) getEnv() *map[string]string {
	return &sal.env
}

func (sal *stepActionLocal) getIfExpression(_ context.Context, stage stepStage) string {
	switch stage {
	case stepStageMain:
		return sal.Step.If.Value
	case stepStagePost:
		return sal.action.Runs.PostIf
	}
	return ""
}

func (sal *stepActionLocal) getActionModel() *model.Action {
	return sal.action
}

func (sal *stepActionLocal) getContainerActionPathsExt(subPath string) (string, string) {
	workdir := sal.RunContext.Config.Workdir
	actionName := normalizePath(subPath)
	containerActionDir := path.Join(sal.RunContext.JobContainer.ToContainerPath(workdir), actionName)
	return actionName, containerActionDir
}

func (sal *stepActionLocal) getContainerActionPaths() (string, string) {
	return sal.getContainerActionPathsExt(sal.Step.Uses)
}

func (sal *stepActionLocal) getTarArchive(ctx context.Context, src string) (io.ReadCloser, error) {
	return sal.RunContext.JobContainer.GetContainerArchive(ctx, src)
}

func (sal *stepActionLocal) getActionPath() string {
	return sal.RunContext.JobContainer.ToContainerPath(path.Join(sal.RunContext.Config.Workdir, sal.Step.Uses))
}

func (sal *stepActionLocal) maybeCopyToActionDir(_ context.Context) error {
	// nothing to do
	return nil
}

func (sal *stepActionLocal) getCompositeRunContext(ctx context.Context) *RunContext {
	if sal.compositeRunContext == nil {
		_, containerActionDir := sal.getContainerActionPaths()

		sal.compositeRunContext = newCompositeRunContext(ctx, sal.RunContext, sal, containerActionDir)
		sal.compositeSteps = sal.compositeRunContext.compositeExecutor(sal.action)
	}
	return sal.compositeRunContext
}

func (sal *stepActionLocal) getCompositeSteps() *compositeSteps {
	return sal.compositeSteps
}
