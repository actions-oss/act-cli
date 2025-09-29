package schema

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	exprparser "github.com/actions-oss/act-cli/internal/expr"
	"gopkg.in/yaml.v3"
)

//go:embed workflow_schema.json
var workflowSchema string

//go:embed action_schema.json
var actionSchema string

var functions = regexp.MustCompile(`^([a-zA-Z0-9_]+)\(([0-9]+),([0-9]+|MAX)\)$`)

type SchemaValidationKind int

const (
	SchemaValidationKindFatal SchemaValidationKind = iota
	SchemaValidationKindWarning
	SchemaValidationKindInvalidProperty
	SchemaValidationKindMismatched
	SchemaValidationKindMissingProperty
)

type Location struct {
	Line   int
	Column int
}

type SchemaValidationError struct {
	Kind SchemaValidationKind
	Location
	Message string
}

func (e SchemaValidationError) Error() string {
	return fmt.Sprintf("Line: %d Column %d: %s", e.Line, e.Column, e.Message)
}

type SchemaValidationErrorCollection struct {
	Errors      []SchemaValidationError
	Collections []SchemaValidationErrorCollection
}

func indent(builder *strings.Builder, in string) {
	for _, v := range strings.Split(in, "\n") {
		if v != "" {
			builder.WriteString("  ")
			builder.WriteString(v)
		}
		builder.WriteString("\n")
	}
}

func (c SchemaValidationErrorCollection) Error() string {
	var builder strings.Builder
	for _, e := range c.Errors {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(e.Error())
	}
	for _, e := range c.Collections {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		indent(&builder, e.Error())
	}
	return builder.String()
}

func (c *SchemaValidationErrorCollection) AddError(err SchemaValidationError) {
	c.Errors = append(c.Errors, err)
}

func AsSchemaValidationErrorCollection(err error) *SchemaValidationErrorCollection {
	if col, ok := err.(SchemaValidationErrorCollection); ok {
		return &col
	}
	if col, ok := err.(*SchemaValidationErrorCollection); ok {
		return col
	}
	if e, ok := err.(SchemaValidationError); ok {
		return &SchemaValidationErrorCollection{
			Errors: []SchemaValidationError{e},
		}
	}
	if e, ok := err.(*SchemaValidationError); ok {
		return &SchemaValidationErrorCollection{
			Errors: []SchemaValidationError{*e},
		}
	}
	return nil
}

type Schema struct {
	Definitions map[string]Definition
}

func (s *Schema) GetDefinition(name string) Definition {
	def, ok := s.Definitions[name]
	if !ok {
		switch name {
		case "any":
			return Definition{OneOf: &[]string{"sequence", "mapping", "number", "boolean", "string", "null"}}
		case "sequence":
			return Definition{Sequence: &SequenceDefinition{ItemType: "any"}}
		case "mapping":
			return Definition{Mapping: &MappingDefinition{LooseKeyType: "any", LooseValueType: "any"}}
		case "number":
			return Definition{Number: &NumberDefinition{}}
		case "string":
			return Definition{String: &StringDefinition{}}
		case "boolean":
			return Definition{Boolean: &BooleanDefinition{}}
		case "null":
			return Definition{Null: &NullDefinition{}}
		}
	}
	return def
}

type Definition struct {
	Context       []string            `json:"context,omitempty"`
	Mapping       *MappingDefinition  `json:"mapping,omitempty"`
	Sequence      *SequenceDefinition `json:"sequence,omitempty"`
	OneOf         *[]string           `json:"one-of,omitempty"`
	AllowedValues *[]string           `json:"allowed-values,omitempty"`
	String        *StringDefinition   `json:"string,omitempty"`
	Number        *NumberDefinition   `json:"number,omitempty"`
	Boolean       *BooleanDefinition  `json:"boolean,omitempty"`
	Null          *NullDefinition     `json:"null,omitempty"`
}

type MappingDefinition struct {
	Properties     map[string]MappingProperty `json:"properties,omitempty"`
	LooseKeyType   string                     `json:"loose-key-type,omitempty"`
	LooseValueType string                     `json:"loose-value-type,omitempty"`
}

type MappingProperty struct {
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
}

func (s *MappingProperty) UnmarshalJSON(data []byte) error {
	if json.Unmarshal(data, &s.Type) != nil {
		type MProp MappingProperty
		return json.Unmarshal(data, (*MProp)(s))
	}
	return nil
}

type SequenceDefinition struct {
	ItemType string `json:"item-type"`
}

type StringDefinition struct {
	Constant     string `json:"constant,omitempty"`
	IsExpression bool   `json:"is-expression,omitempty"`
}

type NumberDefinition struct {
}

type BooleanDefinition struct {
}

type NullDefinition struct {
}

func GetWorkflowSchema() *Schema {
	sh := &Schema{}
	_ = json.Unmarshal([]byte(workflowSchema), sh)
	return sh
}

func GetActionSchema() *Schema {
	sh := &Schema{}
	_ = json.Unmarshal([]byte(actionSchema), sh)
	return sh
}

type Node struct {
	RestrictEval bool
	Definition   string
	Schema       *Schema
	Context      []string
}

type FunctionInfo struct {
	Name string
	Min  int
	Max  int
}

func visitNode(exprNode exprparser.Node, callback func(node exprparser.Node)) {
	callback(exprNode)
	switch node := exprNode.(type) {
	case *exprparser.FunctionNode:
		for _, arg := range node.Args {
			visitNode(arg, callback)
		}
	case *exprparser.UnaryNode:
		visitNode(node.Operand, callback)
	case *exprparser.BinaryNode:
		visitNode(node.Left, callback)
		visitNode(node.Right, callback)
	}
}

func (s *Node) checkSingleExpression(exprNode exprparser.Node) error {
	if len(s.Context) == 0 {
		switch exprNode.(type) {
		case *exprparser.ValueNode:
			return nil
		default:
			return fmt.Errorf("expressions are not allowed here")
		}
	}

	funcs := s.GetFunctions()

	var err error
	visitNode(exprNode, func(node exprparser.Node) {
		if funcCallNode, ok := node.(*exprparser.FunctionNode); ok {
			for _, v := range funcs {
				if strings.EqualFold(funcCallNode.Name, v.Name) {
					if v.Min > len(funcCallNode.Args) {
						err = errors.Join(err, fmt.Errorf("missing parameters for %s expected >= %v got %v", funcCallNode.Name, v.Min, len(funcCallNode.Args)))
					}
					if v.Max < len(funcCallNode.Args) {
						err = errors.Join(err, fmt.Errorf("too many parameters for %s expected <= %v got %v", funcCallNode.Name, v.Max, len(funcCallNode.Args)))
					}
					return
				}
			}
			err = errors.Join(err, fmt.Errorf("unknown Function Call %s", funcCallNode.Name))
		}
		if varNode, ok := node.(*exprparser.ValueNode); ok && varNode.Kind == exprparser.TokenKindNamedValue {
			if str, ok := varNode.Value.(string); ok {
				for _, v := range s.Context {
					if strings.EqualFold(str, v) {
						return
					}
				}
			}
			err = errors.Join(err, fmt.Errorf("unknown Variable Access %v", varNode.Value))
		}
	})
	return err
}

func (s *Node) GetFunctions() []FunctionInfo {
	funcs := []FunctionInfo{}
	AddFunction(&funcs, "contains", 2, 2)
	AddFunction(&funcs, "endsWith", 2, 2)
	AddFunction(&funcs, "format", 1, 255)
	AddFunction(&funcs, "join", 1, 2)
	AddFunction(&funcs, "startsWith", 2, 2)
	AddFunction(&funcs, "toJson", 1, 1)
	AddFunction(&funcs, "fromJson", 1, 1)
	for _, v := range s.Context {
		i := strings.Index(v, "(")
		if i == -1 {
			continue
		}
		smatch := functions.FindStringSubmatch(v)
		if len(smatch) > 0 {
			functionName := smatch[1]
			minParameters, _ := strconv.ParseInt(smatch[2], 10, 32)
			maxParametersRaw := smatch[3]
			var maxParameters int64
			if strings.EqualFold(maxParametersRaw, "MAX") {
				maxParameters = math.MaxInt32
			} else {
				maxParameters, _ = strconv.ParseInt(maxParametersRaw, 10, 32)
			}
			funcs = append(funcs, FunctionInfo{
				Name: functionName,
				Min:  int(minParameters),
				Max:  int(maxParameters),
			})
		}
	}
	return funcs
}

func exprEnd(expr string) int {
	var inQuotes bool
	for i, v := range expr {
		if v == '\'' {
			inQuotes = !inQuotes
		} else if !inQuotes && i+1 < len(expr) && expr[i:i+2] == "}}" {
			return i
		}
	}
	return -1
}

func (s *Node) checkExpression(node *yaml.Node) (bool, error) {
	if s.RestrictEval {
		return false, nil
	}
	val := node.Value
	hadExpr := false
	var err error
	for {
		if i := strings.Index(val, "${{"); i != -1 {
			val = val[i+3:]
		} else {
			return hadExpr, err
		}
		hadExpr = true

		j := exprEnd(val)

		exprNode, parseErr := exprparser.Parse(val[:j])
		if parseErr != nil {
			err = errors.Join(err, SchemaValidationError{
				Location: toLocation(node),
				Message:  fmt.Sprintf("failed to parse: %s", parseErr.Error()),
			})
			continue
		}
		val = val[j+2:]
		cerr := s.checkSingleExpression(exprNode)
		if cerr != nil {
			err = errors.Join(err, SchemaValidationError{
				Location: toLocation(node),
				Message:  cerr.Error(),
			})
		}
	}
}

func AddFunction(funcs *[]FunctionInfo, s string, i1, i2 int) {
	*funcs = append(*funcs, FunctionInfo{
		Name: s,
		Min:  i1,
		Max:  i2,
	})
}

func (s *Node) UnmarshalYAML(node *yaml.Node) error {
	if node != nil && node.Kind == yaml.DocumentNode {
		return s.UnmarshalYAML(node.Content[0])
	}
	def := s.Schema.GetDefinition(s.Definition)
	if s.Context == nil {
		s.Context = def.Context
	}

	isExpr, err := s.checkExpression(node)
	if err != nil {
		return err
	}
	if isExpr {
		return nil
	}
	if def.Mapping != nil {
		return s.checkMapping(node, def)
	} else if def.Sequence != nil {
		return s.checkSequence(node, def)
	} else if def.OneOf != nil {
		return s.checkOneOf(def, node)
	}

	if err := assertKind(node, yaml.ScalarNode); err != nil {
		return err
	}

	if def.String != nil {
		return s.checkString(node, def)
	} else if def.Number != nil {
		var num float64
		return node.Decode(&num)
	} else if def.Boolean != nil {
		var b bool
		return node.Decode(&b)
	} else if def.AllowedValues != nil {
		s := node.Value
		for _, v := range *def.AllowedValues {
			if s == v {
				return nil
			}
		}
		return SchemaValidationError{
			Location: toLocation(node),
			Message:  fmt.Sprintf("expected one of %s got %s", strings.Join(*def.AllowedValues, ","), s),
		}
	} else if def.Null != nil {
		var myNull *byte
		if err := node.Decode(&myNull); err != nil {
			return err
		}
		if myNull != nil {
			return SchemaValidationError{
				Location: toLocation(node),
				Message:  "invalid Null",
			}
		}
		return nil
	}
	return errors.ErrUnsupported
}

func (s *Node) checkString(node *yaml.Node, def Definition) error {
	// caller checks node type
	val := node.Value
	if def.String.Constant != "" && def.String.Constant != val {
		return SchemaValidationError{
			Location: toLocation(node),
			Message:  fmt.Sprintf("expected %s got %s", def.String.Constant, val),
		}
	}
	if def.String.IsExpression && !s.RestrictEval {
		exprNode, parseErr := exprparser.Parse(node.Value)
		if parseErr != nil {
			return SchemaValidationError{
				Location: toLocation(node),
				Message:  fmt.Sprintf("failed to parse: %s", parseErr.Error()),
			}
		}
		cerr := s.checkSingleExpression(exprNode)
		if cerr != nil {
			return SchemaValidationError{
				Location: toLocation(node),
				Message:  cerr.Error(),
			}
		}
	}
	return nil
}

func (s *Node) checkOneOf(def Definition, node *yaml.Node) error {
	var invalidProps = math.MaxInt
	var bestMatches SchemaValidationErrorCollection
	for _, v := range *def.OneOf {
		// Use helper to create child node
		sub := s.childNode(v)
		err := sub.UnmarshalYAML(node)
		if err == nil {
			return nil
		}
		if col := AsSchemaValidationErrorCollection(err); col != nil {
			var matched int
			for _, e := range col.Errors {
				if e.Kind == SchemaValidationKindInvalidProperty {
					matched++
				}
				if e.Kind == SchemaValidationKindMismatched {
					if math.MaxInt == invalidProps {
						bestMatches.Collections = append(bestMatches.Collections, *col)
						continue
					}
				}
			}
			if matched == 0 {
				matched = math.MaxInt
			}
			if matched <= invalidProps {
				if matched < invalidProps {
					// clear, we have better matching ones
					bestMatches.Collections = nil
				}
				bestMatches.Collections = append(bestMatches.Collections, *col)
				invalidProps = matched
			}
			continue
		}
		bestMatches.Errors = append(bestMatches.Errors, SchemaValidationError{
			Location: toLocation(node),
			Message:  fmt.Sprintf("failed to match %s: %s", v, err.Error()),
		})
	}
	if len(bestMatches.Errors) > 0 || len(bestMatches.Collections) > 0 {
		return bestMatches
	}
	return nil
}

func getStringKind(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

func (s *Node) checkSequence(node *yaml.Node, def Definition) error {
	if err := assertKind(node, yaml.SequenceNode); err != nil {
		return err
	}
	var allErrors error
	for _, v := range node.Content {
		// Use helper to create child node
		child := s.childNode(def.Sequence.ItemType)
		allErrors = errors.Join(allErrors, child.UnmarshalYAML(v))
	}
	return allErrors
}

func toLocation(node *yaml.Node) Location {
	return Location{Line: node.Line, Column: node.Column}
}

func assertKind(node *yaml.Node, kind yaml.Kind) error {
	if node.Kind != kind {
		return SchemaValidationError{
			Location: toLocation(node),
			Kind:     SchemaValidationKindMismatched,
			Message:  fmt.Sprintf("expected a %s got %s", getStringKind(kind), getStringKind(node.Kind)),
		}
	}
	return nil
}

func (s *Node) GetNestedNode(path ...string) *Node {
	if s == nil {
		//return nil
		panic("nil")
	}
	if len(path) == 0 {
		return s
	}
	def := s.Schema.GetDefinition(s.Definition)
	if def.Mapping != nil {
		prop, ok := def.Mapping.Properties[path[0]]
		if !ok {
			if def.Mapping.LooseValueType == "" {
				return nil
			}
			return s.childNode(def.Mapping.LooseValueType).GetNestedNode(path[1:]...)
		}
		return s.childNode(prop.Type).GetNestedNode(path[1:]...)
	}
	if def.Sequence != nil {
		// OneOf Branching
		if path[0] != "*" {
			return nil
		}
		return s.childNode(def.Sequence.ItemType).GetNestedNode(path[1:]...)
	}
	if def.OneOf != nil {
		for _, one := range *def.OneOf {
			opt := s.childNode(one).GetNestedNode(path...)
			if opt != nil {
				return opt
			}
		}
		return nil
	}
	return nil
}

func (s *Node) checkMapping(node *yaml.Node, def Definition) error {
	if err := assertKind(node, yaml.MappingNode); err != nil {
		return err
	}
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	var allErrors SchemaValidationErrorCollection
	var hasKeyExpr bool
	usedProperties := map[string]string{}
	for i, k := range node.Content {
		if i%2 == 0 {
			if insertDirective.MatchString(k.Value) {
				if len(s.Context) == 0 {
					allErrors.AddError(SchemaValidationError{
						Location: toLocation(node),
						Message:  "insert is not allowed here",
					})
				}
				hasKeyExpr = true
				continue
			}

			isExpr, err := s.checkExpression(k)
			if err != nil {
				allErrors.AddError(SchemaValidationError{
					Location: toLocation(node),
					Message:  err.Error(),
				})
				hasKeyExpr = true
				continue
			}
			if isExpr {
				hasKeyExpr = true
				continue
			}
			if org, ok := usedProperties[strings.ToLower(k.Value)]; !ok {
				// duplicate check case insensitive
				usedProperties[strings.ToLower(k.Value)] = k.Value
				// schema check case sensitive
				usedProperties[k.Value] = k.Value
			} else {
				allErrors.AddError(SchemaValidationError{
					// Kind:     SchemaValidationKindInvalidProperty,
					Location: toLocation(node),
					Message:  fmt.Sprintf("duplicate property %v of %v", k.Value, org),
				})
			}
			vdef, ok := def.Mapping.Properties[k.Value]
			if !ok {
				if def.Mapping.LooseValueType == "" {
					allErrors.AddError(SchemaValidationError{
						Kind:     SchemaValidationKindInvalidProperty,
						Location: toLocation(node),
						Message:  fmt.Sprintf("unknown property %v", k.Value),
					})
					continue
				}
				vdef = MappingProperty{Type: def.Mapping.LooseValueType}
			}

			// Use helper to create child node
			child := s.childNode(vdef.Type)
			if err := child.UnmarshalYAML(node.Content[i+1]); err != nil {
				if col := AsSchemaValidationErrorCollection(err); col != nil {
					allErrors.AddError(SchemaValidationError{
						Location: toLocation(node.Content[i+1]),
						Message:  fmt.Sprintf("error found in value of key %s", k.Value),
					})
					allErrors.Collections = append(allErrors.Collections, *col)
					continue
				}
				allErrors.AddError(SchemaValidationError{
					Location: toLocation(node),
					Message:  err.Error(),
				})
				continue
			}
		}
	}
	if !hasKeyExpr {
		for k, v := range def.Mapping.Properties {
			if _, ok := usedProperties[k]; !ok && v.Required {
				allErrors.AddError(SchemaValidationError{
					Location: toLocation(node),
					Kind:     SchemaValidationKindMissingProperty,
					Message:  fmt.Sprintf("missing property %s", k),
				})
			}
		}
	}
	if len(allErrors.Errors) == 0 && len(allErrors.Collections) == 0 {
		return nil
	}
	return allErrors
}

func (s *Node) childNode(defName string) *Node {
	return &Node{
		RestrictEval: s.RestrictEval,
		Definition:   defName,
		Schema:       s.Schema,
		Context:      append(append([]string{}, s.Context...), s.Schema.GetDefinition(defName).Context...),
	}
}

func (s *Node) GetVariables() []string {
	// Return only variable names (exclude function signatures)
	vars := []string{}
	for _, v := range s.Context {
		if !strings.Contains(v, "(") {
			vars = append(vars, v)
		}
	}
	return vars
}

// ValidateExpression checks whether all variables and functions used in the expressions
// inside the provided yaml.Node are present in the allowed sets. It returns false
// if any variable or function is missing.
func (s *Node) ValidateExpression(node *yaml.Node, allowedVars map[string]struct{}, allowedFuncs map[string]struct{}) bool {
	val := node.Value
	for {
		i := strings.Index(val, "${{")
		if i == -1 {
			break
		}
		val = val[i+3:]
		j := exprEnd(val)
		exprNode, parseErr := exprparser.Parse(val[:j])
		if parseErr != nil {
			return false
		}
		val = val[j+2:]
		// walk expression tree
		visitNode(exprNode, func(n exprparser.Node) {
			switch el := n.(type) {
			case *exprparser.FunctionNode:
				if _, ok := allowedFuncs[el.Name]; !ok {
					// missing function
					// use a panic to break out
					panic("missing function")
				}
			case *exprparser.ValueNode:
				if el.Kind == exprparser.TokenKindNamedValue {
					if _, ok := allowedVars[el.Value.(string)]; !ok {
						panic("missing variable")
					}
				}
			}
		})
	}
	return true
}
