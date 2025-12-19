package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestAdditionalFunctions(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    if: success() || success('joba', 'jobb') || failure() || failure('joba', 'jobb') || always() || cancelled()
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestAdditionalFunctionsFailure(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    if: success() || success('joba', 'jobb') || failure() || failure('joba', 'jobb') || always('error')
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.Error(t, err)
}

func TestAdditionalFunctionsSteps(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
      if: success() || failure() || always()
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestAdditionalFunctionsStepsExprSyntax(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
      if: ${{ success() || failure() || always() }}
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestFailure(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    x: failure
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.Error(t, err)
}

func TestFailure2(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    Runs-on: failure
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.Error(t, err)
}

func TestEscape(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
${{ 'on' }}: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestSchemaErrors(t *testing.T) {
	table := []struct {
		name  string // test name
		input string // workflow yaml input
		err   string // error message substring
	}{
		{
			name: "case even parameters is error",
			input: `
${{ 'on' }}: push
jobs:
    job-with-condition:
        runs-on: self-hosted
        steps:
        - run: echo ${{ case(1 == 1, 'zero', 2 == 2, 'one', 'two', '') }}
`,
			err: "expected odd number of parameters for case got 6",
		},
		{
			name: "case odd parameters no error",
			input: `
${{ 'on' }}: push
jobs:
    job-with-condition:
        runs-on: self-hosted
        steps:
        - run: echo ${{ case(1 == 1, 'zero', 2 == 2, 'one', 'two') }}
`,
		},
		{
			name: "case 1 parameters error",
			input: `
${{ 'on' }}: push
jobs:
    job-with-condition:
        runs-on: self-hosted
        steps:
        - run: echo ${{ case(1 == 1) }}
`,
			err: "missing parameters for case expected >= 3 got 1",
		},
		{
			name: "invalid expression in step uses",
			input: `
on: push
jobs:
    job-with-condition:
        runs-on: self-hosted
        steps:
        - uses: ${{ format('actions/checkout@v%s', 'v2') }}
`,
			err: "Line: 7 Column 17: expressions are not allowed here",
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(test.input), &node)
			if !assert.NoError(t, err) {
				return
			}
			err = (&Node{
				Definition: "workflow-root-strict",
				Schema:     GetWorkflowSchema(),
			}).UnmarshalYAML(&node)
			if test.err != "" {
				assert.ErrorContains(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestActionSchemaErrors(t *testing.T) {
	table := []struct {
		name  string // test name
		input string // workflow yaml input
		err   string // error message substring
	}{
		{
			name: "missing property shell",
			input: `
runs:
  using: composite
  steps:
  - run: echo failure
`,
			err: "missing property shell",
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(test.input), &node)
			if !assert.NoError(t, err) {
				return
			}
			err = (&Node{
				Definition: "action-root",
				Schema:     GetActionSchema(),
			}).UnmarshalYAML(&node)
			if test.err != "" {
				assert.ErrorContains(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
