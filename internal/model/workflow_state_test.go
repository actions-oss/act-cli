package model

import (
	"context"
	"testing"

	v2 "github.com/actions-oss/act-cli/internal/eval/v2"
	"github.com/actions-oss/act-cli/internal/templateeval"
	"github.com/actions-oss/act-cli/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParseWorkflow(t *testing.T) {
	ee := &templateeval.ExpressionEvaluator{
		EvaluationContext: v2.EvaluationContext{
			Variables: v2.CaseInsensitiveObject[any]{},
			Functions: v2.GetFunctions(),
		},
	}
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
run-name: ${{ fromjson('{}') }}
jobs:
  _:
    name: ${{ github.ref_name }}
    steps:
    - run: echo Hello World
      env:
        TAG: ${{ env.global }}
`), &node)
	require.NoError(t, err)
	err = ee.EvaluateYamlNode(context.Background(), node.Content[0], &schema.Node{
		Definition: "workflow-root",
		Schema:     schema.GetWorkflowSchema(),
	})
	require.NoError(t, err)

	ee.RestrictEval = true
	ee.EvaluationContext.Variables = v2.CaseInsensitiveObject[any]{
		"github": v2.CaseInsensitiveObject[any]{
			"ref_name": "self",
		},
		"vars":   v2.CaseInsensitiveObject[any]{},
		"inputs": v2.CaseInsensitiveObject[any]{},
	}

	err = ee.EvaluateYamlNode(context.Background(), node.Content[0], &schema.Node{
		Definition: "workflow-root",
		Schema:     schema.GetWorkflowSchema(),
	})
	require.Error(t, err)
	var myw Workflow
	require.NoError(t, node.Decode(&myw))
}

func TestParseWorkflowCall(t *testing.T) {
	ee := &templateeval.ExpressionEvaluator{
		EvaluationContext: v2.EvaluationContext{
			Variables: v2.CaseInsensitiveObject[any]{},
			Functions: v2.GetFunctions(),
		},
	}
	var node yaml.Node
	// jobs.test.outputs.test
	err := yaml.Unmarshal([]byte(`
on:
  workflow_call:
    outputs:
      test:
        value: ${{ jobs.test.outputs.test }} # tojson(vars.raw)
run-name: ${{ github.ref_name }}
jobs:
  _:
    runs-on: ubuntu-latest
    name: ${{ github.ref_name }}
    steps:
    - run: echo Hello World
      env:
        TAG: ${{ env.global }}
`), &node)
	require.NoError(t, err)
	require.NoError(t, resolveAliases(node.Content[0]))
	require.NoError(t, (&schema.Node{
		Definition: "workflow-root",
		Schema:     schema.GetWorkflowSchema(),
	}).UnmarshalYAML(node.Content[0]))
	err = ee.EvaluateYamlNode(context.Background(), node.Content[0], &schema.Node{
		Definition: "workflow-root",
		Schema:     schema.GetWorkflowSchema(),
	})
	require.NoError(t, err)

	var raw any
	node.Content[0].Decode(&raw)

	ee.RestrictEval = true
	ee.EvaluationContext.Variables = v2.CaseInsensitiveObject[any]{
		"github": v2.CaseInsensitiveObject[any]{
			"ref_name": "self",
		},
		"vars": v2.CaseInsensitiveObject[any]{
			"raw": raw,
		},
		"inputs": v2.CaseInsensitiveObject[any]{},
		"jobs": v2.CaseInsensitiveObject[any]{
			"test": v2.CaseInsensitiveObject[any]{
				"outputs": v2.CaseInsensitiveObject[any]{
					"test": "Hello World",
				},
			},
		},
	}

	err = ee.EvaluateYamlNode(context.Background(), node.Content[0], &schema.Node{
		RestrictEval: true,
		Definition:   "workflow-root",
		Schema:       schema.GetWorkflowSchema(),
	})
	require.NoError(t, err)
	var myw Workflow
	require.NoError(t, node.Decode(&myw))
	workflow_call := myw.On.WorkflowCall
	if workflow_call != nil {
		for _, out := range workflow_call.Outputs {
			err = ee.EvaluateYamlNode(context.Background(), &out.Value, &schema.Node{
				RestrictEval: true,
				Definition:   "workflow-output-context",
				Schema:       schema.GetWorkflowSchema(),
			})
			require.NoError(t, err)
			require.Equal(t, "Hello World", out.Value.Value)
		}
	}
	out, err := yaml.Marshal(&myw)
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}
