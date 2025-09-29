package templateeval

import (
	"context"
	"testing"

	v2 "github.com/actions-oss/act-cli/internal/eval/v2"
	"github.com/actions-oss/act-cli/pkg/schema"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEval(t *testing.T) {
	ee := &ExpressionEvaluator{
		EvaluationContext: v2.EvaluationContext{
			Variables: v2.CaseInsensitiveObject[any]{},
			Functions: v2.GetFunctions(),
		},
	}
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
run-name: ${{ github.ref_name }}
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
	require.NoError(t, err)
}

func TestEvalWrongType(t *testing.T) {
	ee := &ExpressionEvaluator{
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
}
