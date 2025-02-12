package runner

import (
	"archive/tar"
	"context"
	"fmt"
	"path"
	"regexp"

	"github.com/actions-oss/act-cli/pkg/common"
	"github.com/actions-oss/act-cli/pkg/model"
)

func newLocalReusableWorkflowExecutor(rc *RunContext) common.Executor {
	return newReusableWorkflowExecutor(rc, rc.Config.Workdir, rc.Run.Job().Uses)
}

func newRemoteReusableWorkflowExecutor(rc *RunContext) common.Executor {
	uses := rc.Run.Job().Uses

	remoteReusableWorkflow := newRemoteReusableWorkflow(uses)
	if remoteReusableWorkflow == nil {
		return common.NewErrorExecutor(fmt.Errorf("expected format {owner}/{repo}/.github/workflows/{filename}@{ref}. Actual '%s' Input string was not in a correct format", uses))
	}

	// uses with safe filename makes the target directory look something like this {owner}-{repo}-.github-workflows-{filename}@{ref}
	// instead we will just use {owner}-{repo}@{ref} as our target directory. This should also improve performance when we are using
	// multiple reusable workflows from the same repository and ref since for each workflow we won't have to clone it again
	filename := fmt.Sprintf("%s/%s@%s", remoteReusableWorkflow.Org, remoteReusableWorkflow.Repo, remoteReusableWorkflow.Ref)

	return newActionCacheReusableWorkflowExecutor(rc, filename, remoteReusableWorkflow)
}

func newActionCacheReusableWorkflowExecutor(rc *RunContext, filename string, remoteReusableWorkflow *remoteReusableWorkflow) common.Executor {
	return func(ctx context.Context) error {
		ghctx := rc.getGithubContext(ctx)
		remoteReusableWorkflow.URL = ghctx.ServerURL
		cache := rc.getActionCache()
		sha, err := cache.Fetch(ctx, filename, remoteReusableWorkflow.CloneURL(), remoteReusableWorkflow.Ref, ghctx.Token)
		if err != nil {
			return err
		}
		archive, err := cache.GetTarArchive(ctx, filename, sha, fmt.Sprintf(".github/workflows/%s", remoteReusableWorkflow.Filename))
		if err != nil {
			return err
		}
		defer archive.Close()
		treader := tar.NewReader(archive)
		if _, err = treader.Next(); err != nil {
			return err
		}
		planner, err := model.NewSingleWorkflowPlanner(remoteReusableWorkflow.Filename, treader)
		if err != nil {
			return err
		}
		plan, err := planner.PlanEvent("workflow_call")
		if err != nil {
			return err
		}

		runner, err := NewReusableWorkflowRunner(rc)
		if err != nil {
			return err
		}

		return runner.NewPlanExecutor(plan)(ctx)
	}
}

func newReusableWorkflowExecutor(rc *RunContext, directory string, workflow string) common.Executor {
	return func(ctx context.Context) error {
		planner, err := model.NewWorkflowPlanner(path.Join(directory, workflow), true)
		if err != nil {
			return err
		}

		plan, err := planner.PlanEvent("workflow_call")
		if err != nil {
			return err
		}

		runner, err := NewReusableWorkflowRunner(rc)
		if err != nil {
			return err
		}

		return runner.NewPlanExecutor(plan)(ctx)
	}
}

func NewReusableWorkflowRunner(rc *RunContext) (Runner, error) {
	runner := &runnerImpl{
		config:    rc.Config,
		eventJSON: rc.EventJSON,
		caller: &caller{
			runContext: rc,
		},
	}

	return runner.configure()
}

type remoteReusableWorkflow struct {
	URL      string
	Org      string
	Repo     string
	Filename string
	Ref      string
}

func (r *remoteReusableWorkflow) CloneURL() string {
	return fmt.Sprintf("%s/%s/%s", r.URL, r.Org, r.Repo)
}

func newRemoteReusableWorkflow(uses string) *remoteReusableWorkflow {
	// GitHub docs:
	// https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_iduses
	r := regexp.MustCompile(`^([^/]+)/([^/]+)/.github/workflows/([^@]+)@(.*)$`)
	matches := r.FindStringSubmatch(uses)
	if len(matches) != 5 {
		return nil
	}
	return &remoteReusableWorkflow{
		Org:      matches[1],
		Repo:     matches[2],
		Filename: matches[3],
		Ref:      matches[4],
		URL:      "https://github.com",
	}
}
