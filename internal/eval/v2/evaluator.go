package v2

import (
	"errors"
	"fmt"

	exprparser "github.com/actions-oss/act-cli/internal/expr"
)

// EvaluationContext holds variables that can be referenced in expressions.
type EvaluationContext struct {
	Variables ReadOnlyObject[any]
	Functions ReadOnlyObject[Function]
}

func NewEvaluationContext() *EvaluationContext {
	return &EvaluationContext{}
}

type Function interface {
	Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error)
}

// Evaluator evaluates workflow expressions using the lexer and parser from workflow.
type Evaluator struct {
	ctx *EvaluationContext
}

// NewEvaluator creates an Evaluator with the supplied context.
func NewEvaluator(ctx *EvaluationContext) *Evaluator {
	return &Evaluator{ctx: ctx}
}

func (e *Evaluator) Context() *EvaluationContext {
	return e.ctx
}

func (e *Evaluator) Evaluate(root exprparser.Node) (*EvaluationResult, error) {
	result, err := e.evalNode(root)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// EvaluateBoolean parses and evaluates the expression, returning a boolean result.
func (e *Evaluator) EvaluateBoolean(expr string) (bool, error) {
	root, err := exprparser.Parse(expr)
	if err != nil {
		return false, fmt.Errorf("parse error: %w", err)
	}
	result, err := e.evalNode(root)
	if err != nil {
		return false, err
	}
	return result.IsTruthy(), nil
}

func (e *Evaluator) ToRaw(result *EvaluationResult) (interface{}, error) {
	if col, ok := result.TryGetCollectionInterface(); ok {
		switch node := col.(type) {
		case ReadOnlyObject[any]:
			rawMap := map[string]interface{}{}
			for k, v := range node.GetEnumerator() {
				rawRes, err := e.ToRaw(CreateIntermediateResult(e.Context(), v))
				if err != nil {
					return nil, err
				}
				rawMap[k] = rawRes
			}
			return rawMap, nil
		case ReadOnlyArray[any]:
			rawArray := []interface{}{}
			for _, v := range node.GetEnumerator() {
				rawRes, err := e.ToRaw(CreateIntermediateResult(e.Context(), v))
				if err != nil {
					return nil, err
				}
				rawArray = append(rawArray, rawRes)
			}
			return rawArray, nil
		}
	}
	return result.Value(), nil
}

// Evaluate parses and evaluates the expression, returning a boolean result.
func (e *Evaluator) EvaluateRaw(expr string) (interface{}, error) {
	root, err := exprparser.Parse(expr)
	if err != nil {
		return false, fmt.Errorf("parse error: %w", err)
	}
	result, err := e.evalNode(root)
	if err != nil {
		return false, err
	}
	return e.ToRaw(result)
}

type FilteredArray []interface{}

func (a FilteredArray) GetAt(i int64) interface{} {
	if int(i) > len(a) {
		return nil
	}
	return a[i]
}

func (a FilteredArray) GetEnumerator() []interface{} {
	return a
}

// evalNode recursively evaluates a parser node and returns an EvaluationResult.
func (e *Evaluator) evalNode(n exprparser.Node) (*EvaluationResult, error) {
	switch node := n.(type) {
	case *exprparser.ValueNode:
		if node.Kind == exprparser.TokenKindNamedValue {
			if e.ctx != nil {
				val := e.ctx.Variables.Get(node.Value.(string))
				if val == nil {
					return nil, fmt.Errorf("undefined variable %s", node.Value)
				}
				return CreateIntermediateResult(e.Context(), val), nil
			}
			return nil, errors.New("no evaluation context")
		}
		return CreateIntermediateResult(e.Context(), node.Value), nil
	case *exprparser.FunctionNode:
		fn := e.ctx.Functions.Get(node.Name)
		if fn == nil {
			return nil, fmt.Errorf("unknown function %v", node.Name)
		}
		return fn.Evaluate(e, node.Args)
	case *exprparser.BinaryNode:
		left, err := e.evalNode(node.Left)
		if err != nil {
			return nil, err
		}
		switch node.Op {
		case "&&":
			if left.IsFalsy() {
				return left, nil
			}
		case "||":
			if left.IsTruthy() {
				return left, nil
			}
		case ".":
			if v, ok := node.Right.(*exprparser.ValueNode); ok && v.Kind == exprparser.TokenKindWildcard {
				var ret FilteredArray
				if col, ok := left.TryGetCollectionInterface(); ok {
					if farray, ok := col.(FilteredArray); ok {
						for _, subcol := range farray.GetEnumerator() {
							ret = processStar(CreateIntermediateResult(e.Context(), subcol).Value(), ret)
						}
					} else {
						ret = processStar(col, ret)
					}
				}
				return CreateIntermediateResult(e.Context(), ret), nil
			}
		}
		right, err := e.evalNode(node.Right)
		if err != nil {
			return nil, err
		}
		switch node.Op {
		case "&&":
			return right, nil
		case "||":
			return right, nil
		case "==":
			// Use abstract equality per spec
			return CreateIntermediateResult(e.Context(), left.AbstractEqual(right)), nil
		case "!=":
			return CreateIntermediateResult(e.Context(), left.AbstractNotEqual(right)), nil
		case ">":
			return CreateIntermediateResult(e.Context(), left.AbstractGreaterThan(right)), nil
		case "<":
			return CreateIntermediateResult(e.Context(), left.AbstractLessThan(right)), nil
		case ">=":
			return CreateIntermediateResult(e.Context(), left.AbstractGreaterThanOrEqual(right)), nil
		case "<=":
			return CreateIntermediateResult(e.Context(), left.AbstractLessThanOrEqual(right)), nil
		case ".", "[":
			if farray, ok := left.Value().(FilteredArray); ok {
				var ret FilteredArray
				for _, subcol := range farray.GetEnumerator() {
					res := processIndex(CreateIntermediateResult(e.Context(), subcol).Value(), right)
					if res != nil {
						ret = append(ret, res)
					}
				}
				if ret == nil {
					return CreateIntermediateResult(e.Context(), nil), nil
				}
				return CreateIntermediateResult(e.Context(), ret), nil
			}
			col, _ := left.TryGetCollectionInterface()
			result := processIndex(col, right)
			return CreateIntermediateResult(e.Context(), result), nil
		default:
			return nil, fmt.Errorf("unsupported operator %s", node.Op)
		}
	case *exprparser.UnaryNode:
		operand, err := e.evalNode(node.Operand)
		if err != nil {
			return nil, err
		}
		switch node.Op {
		case "!":
			return CreateIntermediateResult(e.Context(), !operand.IsTruthy()), nil
		default:
			return nil, fmt.Errorf("unsupported unary operator %s", node.Op)
		}
	}
	return nil, errors.New("unknown node type")
}

func processIndex(col interface{}, right *EvaluationResult) interface{} {
	if mapVal, ok := col.(ReadOnlyObject[any]); ok {
		key, ok := right.Value().(string)
		if !ok {
			return nil
		}
		val := mapVal.Get(key)
		return val
	}
	if arrayVal, ok := col.(ReadOnlyArray[any]); ok {
		key, ok := right.Value().(float64)
		if !ok || key < 0 {
			return nil
		}
		val := arrayVal.GetAt(int64(key))
		return val
	}
	return nil
}

func processStar(subcol interface{}, ret FilteredArray) FilteredArray {
	if array, ok := subcol.(ReadOnlyArray[any]); ok {
		ret = append(ret, array.GetEnumerator()...)
	} else if obj, ok := subcol.(ReadOnlyObject[any]); ok {
		for _, v := range obj.GetEnumerator() {
			ret = append(ret, v)
		}
	}
	return ret
}
