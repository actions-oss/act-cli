package exprparser

import (
	"encoding"
	"fmt"
	"math"
	"reflect"
	"strings"

	eval "github.com/actions-oss/act-cli/internal/eval/v2"
	exprparser "github.com/actions-oss/act-cli/internal/expr"
	"github.com/actions-oss/act-cli/pkg/model"
)

type EvaluationEnvironment struct {
	Github    *model.GithubContext
	Env       map[string]string
	Job       *model.JobContext
	Jobs      *map[string]*model.WorkflowCallResult
	Steps     map[string]*model.StepResult
	Runner    map[string]interface{}
	Secrets   map[string]string
	Vars      map[string]string
	Strategy  map[string]interface{}
	Matrix    map[string]interface{}
	Needs     map[string]Needs
	Inputs    map[string]interface{}
	HashFiles func([]reflect.Value) (interface{}, error)
	EnvCS     bool
	CtxData   map[string]interface{}
}

type CaseSensitiveDict map[string]string

type Needs struct {
	Outputs map[string]string `json:"outputs"`
	Result  string            `json:"result"`
}

type Config struct {
	Run        *model.Run
	WorkingDir string
	Context    string
}

type DefaultStatusCheck int

const (
	DefaultStatusCheckNone DefaultStatusCheck = iota
	DefaultStatusCheckSuccess
	DefaultStatusCheckAlways
	DefaultStatusCheckCanceled
	DefaultStatusCheckFailure
)

func (dsc DefaultStatusCheck) String() string {
	switch dsc {
	case DefaultStatusCheckSuccess:
		return "success"
	case DefaultStatusCheckAlways:
		return "always"
	case DefaultStatusCheckCanceled:
		return "cancelled"
	case DefaultStatusCheckFailure:
		return "failure"
	}
	return ""
}

type Interpreter interface {
	Evaluate(input string, defaultStatusCheck DefaultStatusCheck) (interface{}, error)
}

type interperterImpl struct {
	env    *EvaluationEnvironment
	config Config
}

func NewInterpeter(env *EvaluationEnvironment, config Config) Interpreter {
	return &interperterImpl{
		env:    env,
		config: config,
	}
}

func toRawObj(left reflect.Value) map[string]any {
	res, _ := toRaw(left).(map[string]any)
	return res
}

func toRaw(left reflect.Value) any {
	switch left.Kind() {
	case reflect.Pointer:
		if left.IsNil() {
			return nil
		}
		return toRaw(left.Elem())
	case reflect.Map:
		iter := left.MapRange()

		m := map[string]any{}

		for iter.Next() {
			key := iter.Key()

			switch key.Kind() {
			case reflect.String:
				m[key.String()] = toRaw(iter.Value())
			}
		}
		return m
	case reflect.Struct:
		m := map[string]any{}

		leftType := left.Type()
		for i := 0; i < leftType.NumField(); i++ {
			var name string
			if jsonName := leftType.Field(i).Tag.Get("json"); jsonName != "" {
				name, _, _ = strings.Cut(jsonName, ",")
			}
			if name == "" {
				name = leftType.Field(i).Name
			}
			v := left.Field(i).Interface()
			if t, ok := v.(encoding.TextMarshaler); ok {
				text, _ := t.MarshalText()
				m[name] = string(text)
			} else {
				m[name] = toRaw(left.Field(i))
			}
		}

		return m
	}
	return left.Interface()
}

// All values are evaluated as string, funcs that takes objects are implemented elsewhere
type externalFunc struct {
	f func([]reflect.Value) (interface{}, error)
}

func (e externalFunc) Evaluate(ev *eval.Evaluator, args []exprparser.Node) (*eval.EvaluationResult, error) {
	rargs := []reflect.Value{}
	for _, arg := range args {
		res, err := ev.Evaluate(arg)
		if err != nil {
			return nil, err
		}
		rargs = append(rargs, reflect.ValueOf(res.ConvertToString()))
	}
	res, err := e.f(rargs)
	if err != nil {
		return nil, err
	}
	return eval.CreateIntermediateResult(ev.Context(), res), nil
}

func (impl *interperterImpl) Evaluate(input string, defaultStatusCheck DefaultStatusCheck) (interface{}, error) {
	input = strings.TrimPrefix(input, "${{")
	input = strings.TrimSuffix(input, "}}")
	if defaultStatusCheck != DefaultStatusCheckNone && input == "" {
		input = "success()"
	}

	exprNode, err := exprparser.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %s", err.Error())
	}

	if defaultStatusCheck != DefaultStatusCheckNone {
		hasStatusCheckFunction := false
		exprparser.VisitNode(exprNode, func(node exprparser.Node) {
			if funcCallNode, ok := node.(*exprparser.FunctionNode); ok {
				switch strings.ToLower(funcCallNode.Name) {
				case "success", "always", "cancelled", "failure":
					hasStatusCheckFunction = true
				}
			}
		})

		if !hasStatusCheckFunction {
			exprNode = &exprparser.BinaryNode{
				Op: "&&",
				Left: &exprparser.FunctionNode{
					Name: defaultStatusCheck.String(),
					Args: []exprparser.Node{},
				},
				Right: exprNode,
			}
		}
	}

	functions := eval.GetFunctions()
	if impl.env.HashFiles != nil {
		functions["hashfiles"] = &externalFunc{impl.env.HashFiles}
	}
	functions["always"] = &externalFunc{func(v []reflect.Value) (interface{}, error) {
		return impl.always()
	}}
	functions["success"] = &externalFunc{func(v []reflect.Value) (interface{}, error) {
		if impl.config.Context == "job" {
			return impl.jobSuccess()
		}
		if impl.config.Context == "step" {
			return impl.stepSuccess()
		}
		return nil, fmt.Errorf("context '%s' must be one of 'job' or 'step'", impl.config.Context)
	}}
	functions["failure"] = &externalFunc{func(v []reflect.Value) (interface{}, error) {
		if impl.config.Context == "job" {
			return impl.jobFailure()
		}
		if impl.config.Context == "step" {
			return impl.stepFailure()
		}
		return nil, fmt.Errorf("context '%s' must be one of 'job' or 'step'", impl.config.Context)
	}}
	functions["cancelled"] = &externalFunc{func(v []reflect.Value) (interface{}, error) {
		return impl.cancelled()
	}}

	githubCtx := toRawObj(reflect.ValueOf(impl.env.Github))
	var env any
	if impl.env.EnvCS {
		env = eval.CaseSensitiveObject[any](toRawObj(reflect.ValueOf(impl.env.Env)))
	} else {
		env = eval.CaseInsensitiveObject[any](toRawObj(reflect.ValueOf(impl.env.Env)))
	}
	vars := eval.CaseInsensitiveObject[any]{
		"github":   githubCtx,
		"env":      env,
		"vars":     toRawObj(reflect.ValueOf(impl.env.Vars)),
		"steps":    toRawObj(reflect.ValueOf(impl.env.Steps)),
		"strategy": toRawObj(reflect.ValueOf(impl.env.Strategy)),
		"matrix":   toRawObj(reflect.ValueOf(impl.env.Matrix)),
		"secrets":  toRawObj(reflect.ValueOf(impl.env.Secrets)),
		"job":      toRawObj(reflect.ValueOf(impl.env.Job)),
		"runner":   toRawObj(reflect.ValueOf(impl.env.Runner)),
		"needs":    toRawObj(reflect.ValueOf(impl.env.Needs)),
		"jobs":     toRawObj(reflect.ValueOf(impl.env.Jobs)),
		"inputs":   toRawObj(reflect.ValueOf(impl.env.Inputs)),
	}
	for name, cd := range impl.env.CtxData {
		lowerName := strings.ToLower(name)
		if serverPayload, ok := cd.(map[string]interface{}); ok {
			if lowerName == "github" {
				for k, v := range serverPayload {
					// skip empty values, because github.workspace was set by Gitea Actions to an empty string
					if _, ok := githubCtx[k]; !ok || v != "" && v != nil {
						githubCtx[k] = v
					}
				}
				continue
			}
		}
		vars[name] = cd
	}

	ctx := eval.EvaluationContext{
		Functions: functions,
		Variables: vars,
	}
	evaluator := eval.NewEvaluator(&ctx)
	res, err := evaluator.Evaluate(exprNode)
	if err != nil {
		return nil, err
	}
	return evaluator.ToRaw(res)
}

func IsTruthy(input interface{}) bool {
	value := reflect.ValueOf(input)
	switch value.Kind() {
	case reflect.Bool:
		return value.Bool()

	case reflect.String:
		return value.String() != ""

	case reflect.Int:
		return value.Int() != 0

	case reflect.Float64:
		if math.IsNaN(value.Float()) {
			return false
		}

		return value.Float() != 0

	case reflect.Map, reflect.Slice:
		return true

	default:
		return false
	}
}
