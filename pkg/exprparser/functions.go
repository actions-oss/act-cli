package exprparser

import "github.com/actions-oss/act-cli/pkg/model"

func (impl *interperterImpl) getNeedsTransitive(job *model.Job) []string {
	needs := job.Needs()

	for _, need := range needs {
		parentNeeds := impl.getNeedsTransitive(impl.config.Run.Workflow.GetJob(need))
		needs = append(needs, parentNeeds...)
	}

	return needs
}

func (impl *interperterImpl) always() (interface{}, error) {
	return true, nil
}

func (impl *interperterImpl) jobSuccess() (interface{}, error) {
	jobs := impl.config.Run.Workflow.Jobs
	jobNeeds := impl.getNeedsTransitive(impl.config.Run.Job())

	for _, needs := range jobNeeds {
		if jobs[needs].Result != "success" {
			return false, nil
		}
	}

	return true, nil
}

func (impl *interperterImpl) stepSuccess() (interface{}, error) {
	return impl.env.Job.Status == "success", nil
}

func (impl *interperterImpl) jobFailure() (interface{}, error) {
	jobs := impl.config.Run.Workflow.Jobs
	jobNeeds := impl.getNeedsTransitive(impl.config.Run.Job())

	for _, needs := range jobNeeds {
		if jobs[needs].Result == "failure" {
			return true, nil
		}
	}

	return false, nil
}

func (impl *interperterImpl) stepFailure() (interface{}, error) {
	return impl.env.Job.Status == "failure", nil
}

func (impl *interperterImpl) cancelled() (interface{}, error) {
	return impl.env.Job.Status == "cancelled", nil
}
