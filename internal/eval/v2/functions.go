package v2

import (
	"encoding/json"
	"strings"

	"github.com/actions-oss/act-cli/internal/eval/functions"
	exprparser "github.com/actions-oss/act-cli/internal/expr"
)

type FromJSON struct {
}

func (FromJSON) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	r, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}
	var res any
	if err := json.Unmarshal([]byte(r.ConvertToString()), &res); err != nil {
		return nil, err
	}

	return CreateIntermediateResult(eval.Context(), res), nil
}

type ToJSON struct {
}

func (ToJSON) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	r, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}
	raw, err := eval.ToRaw(r)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(raw, "", "    ")
	if err != nil {
		return nil, err
	}
	return CreateIntermediateResult(eval.Context(), string(data)), nil
}

type Contains struct {
}

func (Contains) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	collection, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}
	el, err := eval.Evaluate(args[1])
	if err != nil {
		return nil, err
	}
	// Array
	if col, ok := collection.TryGetCollectionInterface(); ok {
		switch node := col.(type) {
		case ReadOnlyArray[any]:
			for _, v := range node.GetEnumerator() {
				canon := CreateIntermediateResult(eval.Context(), v)
				if canon.AbstractEqual(el) {
					return CreateIntermediateResult(eval.Context(), true), nil
				}
			}
		}
		return CreateIntermediateResult(eval.Context(), false), nil
	}
	// String
	return CreateIntermediateResult(eval.Context(), strings.Contains(strings.ToLower(collection.ConvertToString()), strings.ToLower(el.ConvertToString()))), nil
}

type StartsWith struct {
}

func (StartsWith) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	collection, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}
	el, err := eval.Evaluate(args[1])
	if err != nil {
		return nil, err
	}
	// String
	return CreateIntermediateResult(eval.Context(), strings.HasPrefix(strings.ToLower(collection.ConvertToString()), strings.ToLower(el.ConvertToString()))), nil
}

type EndsWith struct {
}

func (EndsWith) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	collection, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}
	el, err := eval.Evaluate(args[1])
	if err != nil {
		return nil, err
	}
	// String
	return CreateIntermediateResult(eval.Context(), strings.HasSuffix(strings.ToLower(collection.ConvertToString()), strings.ToLower(el.ConvertToString()))), nil
}

type Format struct {
}

func (Format) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	collection, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}

	sargs := []interface{}{}
	for _, arg := range args[1:] {
		el, err := eval.Evaluate(arg)
		if err != nil {
			return nil, err
		}
		sargs = append(sargs, el.ConvertToString())
	}

	ret, err := functions.Format(collection.ConvertToString(), sargs...)
	return CreateIntermediateResult(eval.Context(), ret), err
}

type Join struct {
}

func (Join) Evaluate(eval *Evaluator, args []exprparser.Node) (*EvaluationResult, error) {
	collection, err := eval.Evaluate(args[0])
	if err != nil {
		return nil, err
	}
	var el *EvaluationResult

	if len(args) > 1 {
		if el, err = eval.Evaluate(args[1]); err != nil {
			return nil, err
		}
	}
	// Array
	if col, ok := collection.TryGetCollectionInterface(); ok {
		switch node := col.(type) {
		case ReadOnlyArray[any]:
			for _, v := range node.GetEnumerator() {
				canon := CreateIntermediateResult(eval.Context(), v)
				if canon.AbstractEqual(el) {
					return CreateIntermediateResult(eval.Context(), true), nil
				}
			}
		}
		return CreateIntermediateResult(eval.Context(), ""), nil
	}
	return CreateIntermediateResult(eval.Context(), nil), nil
}

func GetFunctions() CaseInsensitiveObject[Function] {
	return CaseInsensitiveObject[Function](map[string]Function{
		"fromjson":   &FromJSON{},
		"tojson":     &ToJSON{},
		"contains":   &Contains{},
		"startswith": &StartsWith{},
		"endswith":   &EndsWith{},
		"format":     &Format{},
		"join":       &Join{},
	})
}
