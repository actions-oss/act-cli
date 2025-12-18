package templateeval

import (
	"context"
	"fmt"
	"regexp"

	v2 "github.com/actions-oss/act-cli/internal/eval/v2"
	exprparser "github.com/actions-oss/act-cli/internal/expr"
	"github.com/actions-oss/act-cli/pkg/schema"
	"gopkg.in/yaml.v3"
)

type ExpressionEvaluator struct {
	RestrictEval      bool
	EvaluationContext v2.EvaluationContext
}

func isImplExpr(snode *schema.Node) bool {
	def := snode.Schema.GetDefinition(snode.Definition)
	return def.String != nil && def.String.IsExpression
}

func (ee ExpressionEvaluator) evaluateScalarYamlNode(_ context.Context, node *yaml.Node, snode *schema.Node) (*yaml.Node, error) {
	var in string
	if err := node.Decode(&in); err != nil {
		return nil, err
	}
	expr, isExpr, err := rewriteSubExpression(in, false)
	if err != nil {
		return nil, err
	}
	if snode == nil || !isExpr && !isImplExpr(snode) || snode.Schema.GetDefinition(snode.Definition).String.IsExpression || ee.RestrictEval && node.Tag != "!!expr" {
		return node, nil
	}
	parsed, err := exprparser.Parse(expr)
	if err != nil {
		return nil, err
	}
	canEvaluate := ee.canEvaluate(parsed, snode)
	if !canEvaluate {
		node.Tag = "!!expr"
		return node, nil
	}

	eval := v2.NewEvaluator(&ee.EvaluationContext)
	res, err := eval.EvaluateRaw(expr)
	if err != nil {
		return nil, err
	}
	ret := &yaml.Node{}
	if err := ret.Encode(res); err != nil {
		return nil, err
	}
	ret.Line = node.Line
	ret.Column = node.Column
	// Finally check if we found a schema validation error
	return ret, snode.UnmarshalYAML(ret)
}

func (ee ExpressionEvaluator) canEvaluate(parsed exprparser.Node, snode *schema.Node) bool {
	canEvaluate := true
	for _, v := range snode.GetVariables() {
		canEvaluate = canEvaluate && ee.EvaluationContext.Variables.Get(v) != nil
	}
	for _, v := range snode.GetFunctions() {
		canEvaluate = canEvaluate && ee.EvaluationContext.Functions.Get(v.GetName()) != nil
	}
	exprparser.VisitNode(parsed, func(node exprparser.Node) {
		switch el := node.(type) {
		case *exprparser.FunctionNode:
			canEvaluate = canEvaluate && ee.EvaluationContext.Functions.Get(el.Name) != nil
		case *exprparser.ValueNode:
			canEvaluate = canEvaluate && (el.Kind != exprparser.TokenKindNamedValue || ee.EvaluationContext.Variables.Get(el.Value.(string)) != nil)
		}
	})
	return canEvaluate
}

func (ee ExpressionEvaluator) evaluateMappingYamlNode(ctx context.Context, node *yaml.Node, snode *schema.Node) (*yaml.Node, error) {
	var ret *yaml.Node
	// GitHub has this undocumented feature to merge maps, called insert directive
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	for i := 0; i < len(node.Content)/2; i++ {
		k := node.Content[i*2]
		var sk string
		shouldInsert := k.Decode(&sk) == nil && insertDirective.MatchString(sk)
		changed := func() error {
			if ret == nil {
				ret = &yaml.Node{}
				if err := ret.Encode(node); err != nil {
					return err
				}
				ret.Content = ret.Content[:i*2]
			}
			return nil
		}
		var ek *yaml.Node
		if !shouldInsert {
			var err error
			ek, err = ee.evaluateYamlNodeInternal(ctx, k, snode)
			if err != nil {
				return nil, err
			}
			if ek != nil {
				if err := changed(); err != nil {
					return nil, err
				}
			} else {
				ek = k
			}
		}
		v := node.Content[i*2+1]
		ev, err := ee.evaluateYamlNodeInternal(ctx, v, snode.GetNestedNode(ek.Value))
		if err != nil {
			return nil, err
		}
		if ev != nil {
			if err := changed(); err != nil {
				return nil, err
			}
		} else {
			ev = v
		}
		// Merge the nested map of the insert directive
		if shouldInsert {
			if ev.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("failed to insert node %v into mapping %v unexpected type %v expected MappingNode", ev, node, ev.Kind)
			}
			if err := changed(); err != nil {
				return nil, err
			}
			ret.Content = append(ret.Content, ev.Content...)
		} else if ret != nil {
			ret.Content = append(ret.Content, ek, ev)
		}
	}
	return ret, nil
}

func (ee ExpressionEvaluator) evaluateSequenceYamlNode(ctx context.Context, node *yaml.Node, snode *schema.Node) (*yaml.Node, error) {
	var ret *yaml.Node
	for i := 0; i < len(node.Content); i++ {
		v := node.Content[i]
		// Preserve nested sequences
		wasseq := v.Kind == yaml.SequenceNode
		ev, err := ee.evaluateYamlNodeInternal(ctx, v, snode.GetNestedNode("*"))
		if err != nil {
			return nil, err
		}
		if ev != nil {
			if ret == nil {
				ret = &yaml.Node{}
				if err := ret.Encode(node); err != nil {
					return nil, err
				}
				ret.Content = ret.Content[:i]
			}
			// GitHub has this undocumented feature to merge sequences / arrays
			// We have a nested sequence via evaluation, merge the arrays
			if ev.Kind == yaml.SequenceNode && !wasseq {
				ret.Content = append(ret.Content, ev.Content...)
			} else {
				ret.Content = append(ret.Content, ev)
			}
		} else if ret != nil {
			ret.Content = append(ret.Content, v)
		}
	}
	return ret, nil
}

func (ee ExpressionEvaluator) evaluateYamlNodeInternal(ctx context.Context, node *yaml.Node, snode *schema.Node) (*yaml.Node, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return ee.evaluateScalarYamlNode(ctx, node, snode)
	case yaml.MappingNode:
		return ee.evaluateMappingYamlNode(ctx, node, snode)
	case yaml.SequenceNode:
		return ee.evaluateSequenceYamlNode(ctx, node, snode)
	default:
		return nil, nil
	}
}

func (ee ExpressionEvaluator) EvaluateYamlNode(ctx context.Context, node *yaml.Node, snode *schema.Node) error {
	ret, err := ee.evaluateYamlNodeInternal(ctx, node, snode)
	if err != nil {
		return err
	}
	if ret != nil {
		return ret.Decode(node)
	}
	return nil
}
