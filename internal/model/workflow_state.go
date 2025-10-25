package model

import "go.yaml.in/yaml/v4"

type JobStatus int

const (
	JobStatusPending JobStatus = iota
	JobStatusDependenciesReady
	JobStatusBlocked
	JobStatusCompleted
)

type JobState struct {
	JobID    string            // Workflow path to job, incl matrix and parent jobids
	Result   string            // Actions Job Result
	Outputs  map[string]string // Returned Outputs
	State    JobStatus
	Strategy []MatrixJobState
}

type MatrixJobState struct {
	Matrix  map[string]any
	Name    string
	Result  string
	Outputs map[string]string // Returned Outputs
	State   JobStatus
}

type WorkflowStatus int

const (
	WorkflowStatusPending WorkflowStatus = iota
	WorkflowStatusDependenciesReady
	WorkflowStatusBlocked
	WorkflowStatusCompleted
)

type WorkflowState struct {
	Name                string
	RunName             string
	Jobs                JobState
	StateWorkflowStatus WorkflowStatus
}

type Workflow struct {
	On          *On            `yaml:"on,omitempty"`
	Name        string         `yaml:"name,omitempty"`
	Description string         `yaml:"description,omitempty"`
	RunName     yaml.Node      `yaml:"run-name,omitempty"`
	Permissions *Permissions   `yaml:"permissions,omitempty"`
	Env         yaml.Node      `yaml:"env,omitempty"`
	Defaults    yaml.Node      `yaml:"defaults,omitempty"`
	Concurrency yaml.Node      `yaml:"concurrency,omitempty"` // Two layouts
	Jobs        map[string]Job `yaml:"jobs,omitempty"`
}

type On struct {
	Data             map[string]yaml.Node `yaml:"-"`
	WorkflowDispatch *WorkflowDispatch    `yaml:"workflow_dispatch,omitempty"`
	WorkflowCall     *WorkflowCall        `yaml:"workflow_call,omitempty"`
	Schedule         []Cron               `yaml:"schedule,omitempty"`
}

type Cron struct {
	Cron string `yaml:"cron,omitempty"`
}

func (a *On) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		a.Data = map[string]yaml.Node{}
		a.Data[s] = yaml.Node{}
	case yaml.SequenceNode:
		var s []string
		if err := node.Decode(&s); err != nil {
			return err
		}
		a.Data = map[string]yaml.Node{}
		for _, v := range s {
			a.Data[v] = yaml.Node{}
		}
	default:
		if err := node.Decode(&a.Data); err != nil {
			return err
		}
		type OnObj On
		if err := node.Decode((*OnObj)(a)); err != nil {
			return err
		}
	}
	return nil
}

func (a *On) MarshalYAML() (interface{}, error) {
	return a.Data, nil
}

var (
	_ yaml.Unmarshaler = &On{}
	_ yaml.Marshaler   = &On{}
	_ yaml.Unmarshaler = &Concurrency{}
	_ yaml.Unmarshaler = &RunsOn{}
	_ yaml.Unmarshaler = &ImplicitStringArray{}
	_ yaml.Unmarshaler = &Environment{}
)

type WorkflowDispatch struct {
	Inputs map[string]Input `yaml:"inputs,omitempty"`
}

type Input struct {
	Description string `yaml:"description,omitempty"`
	Type        string `yaml:"type,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

type WorkflowCall struct {
	Inputs  map[string]Input  `yaml:"inputs,omitempty"`
	Secrets map[string]Secret `yaml:"secrets,omitempty"`
	Outputs map[string]Output `yaml:"outputs,omitempty"`
}

type Secret struct {
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

type Output struct {
	Description string    `yaml:"description,omitempty"`
	Value       yaml.Node `yaml:"value,omitempty"`
}

type Job struct {
	Needs       ImplicitStringArray `yaml:"needs,omitempty"`
	Permissions *Permissions        `yaml:"permissions,omitempty"`
	Strategy    yaml.Node           `yaml:"strategy,omitempty"`
	Name        yaml.Node           `yaml:"name,omitempty"`
	Concurrency yaml.Node           `yaml:"concurrency,omitempty"`
	// Reusable Workflow
	Uses    yaml.Node `yaml:"uses,omitempty"`
	With    yaml.Node `yaml:"with,omitempty"`
	Secrets yaml.Node `yaml:"secrets,omitempty"`
	// Runner Job
	RunsOn         yaml.Node   `yaml:"runs-on,omitempty"`
	Defaults       yaml.Node   `yaml:"defaults,omitempty"`
	TimeoutMinutes yaml.Node   `yaml:"timeout-minutes,omitempty"`
	Container      yaml.Node   `yaml:"container,omitempty"`
	Services       yaml.Node   `yaml:"services,omitempty"`
	Env            yaml.Node   `yaml:"env,omitempty"`
	Steps          []yaml.Node `yaml:"steps,omitempty"`
	Outputs        yaml.Node   `yaml:"outputs,omitempty"`
}

type ImplicitStringArray []string

func (a *ImplicitStringArray) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*a = []string{s}
		return nil
	}
	return node.Decode((*[]string)(a))
}

type Permissions map[string]string

func (p *Permissions) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		var perm string
		switch s {
		case "read-all":
			perm = "read"
		case "write-all":
			perm = "write"
		default:
			return nil
		}
		(*p)["actions"] = perm
		(*p)["attestations"] = perm
		(*p)["contents"] = perm
		(*p)["checks"] = perm
		(*p)["deployments"] = perm
		(*p)["discussions"] = perm
		(*p)["id-token"] = perm
		(*p)["issues"] = perm
		(*p)["models"] = perm
		(*p)["packages"] = perm
		(*p)["pages"] = perm
		(*p)["pull-requests"] = perm
		(*p)["repository-projects"] = perm
		(*p)["security-events"] = perm
		(*p)["statuses"] = perm
		return nil
	}
	return node.Decode((*map[string]string)(p))
}

type Strategy struct {
	Matrix      map[string][]yaml.Node `yaml:"matrix"`
	MaxParallel float64                `yaml:"max-parallel"`
	FailFast    bool                   `yaml:"fail-fast"`
}

type Concurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
}

func (c *Concurrency) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		c.Group = s
		return nil
	}
	type ConcurrencyObj Concurrency
	return node.Decode((*ConcurrencyObj)(c))
}

type Environment struct {
	Name string    `yaml:"name"`
	URL  yaml.Node `yaml:"url"`
}

func (e *Environment) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		e.Name = s
		return nil
	}
	type EnvironmentObj Environment
	return node.Decode((*EnvironmentObj)(e))
}

type RunsOn struct {
	Labels []string `yaml:"labels"`
	Group  string   `yaml:"group,omitempty"`
}

func (a *RunsOn) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		a.Labels = []string{s}
		return nil
	}
	if node.Kind == yaml.SequenceNode {
		var s []string
		if err := node.Decode(&s); err != nil {
			return err
		}
		a.Labels = s
		return nil
	}
	type RunsOnObj RunsOn
	return node.Decode((*RunsOnObj)(a))
}
