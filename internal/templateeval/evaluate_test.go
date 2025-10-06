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
	cases := []struct {
		name      string
		yamlInput string
		restrict  bool
		variables v2.CaseInsensitiveObject[any]
		expectErr bool
	}{
		{
			name: "NoError",
			yamlInput: `on: push
run-name: ${{ github.ref_name }}
jobs:
  _:
    name: ${{ github.ref_name }}
    steps:
    - run: echo Hello World
      env:
        TAG: ${{ env.global }}`,
			restrict:  false,
			expectErr: false,
		},
		{
			name: "Error",
			yamlInput: `on: push
run-name: ${{ fromjson('{}') }}
jobs:
  _:
    name: ${{ github.ref_name }}
    steps:
    - run: echo Hello World
      env:
        TAG: ${{ env.global }}`,
			restrict: true,
			variables: v2.CaseInsensitiveObject[any]{
				"github": v2.CaseInsensitiveObject[any]{
					"ref_name": "self",
				},
				"vars":   v2.CaseInsensitiveObject[any]{},
				"inputs": v2.CaseInsensitiveObject[any]{},
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ee := &ExpressionEvaluator{
				EvaluationContext: v2.EvaluationContext{
					Variables: v2.CaseInsensitiveObject[any]{},
					Functions: v2.GetFunctions(),
				},
			}
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tc.yamlInput), &node)
			require.NoError(t, err)

			err = ee.EvaluateYamlNode(context.Background(), node.Content[0], &schema.Node{
				Definition: "workflow-root",
				Schema:     schema.GetWorkflowSchema(),
			})
			require.NoError(t, err)

			if tc.restrict {
				ee.RestrictEval = true
			}
			if tc.variables != nil {
				ee.EvaluationContext.Variables = tc.variables
			}

			err = ee.EvaluateYamlNode(context.Background(), node.Content[0], &schema.Node{
				Definition: "workflow-root",
				Schema:     schema.GetWorkflowSchema(),
			})
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
