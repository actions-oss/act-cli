package model

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// TraceWriter is an interface for logging trace information.
// Implementations can write to console, file, or any other sink.
type TraceWriter interface {
	Info(format string, args ...interface{})
}

// StrategyResult holds the result of expanding a strategy.
// FlatMatrix contains the expanded matrix entries.
// IncludeMatrix contains entries that were added via include.
// FailFast indicates whether the job should fail fast.
// MaxParallel is the maximum parallelism allowed.
// MatrixKeys is the set of keys present in the matrix.
type StrategyResult struct {
	FlatMatrix    []map[string]yaml.Node
	IncludeMatrix []map[string]yaml.Node
	FailFast      bool
	MaxParallel   *float64
	MatrixKeys    map[string]struct{}
}

type strategyContext struct {
	jobTraceWriter TraceWriter
	failFast       bool
	maxParallel    float64
	matrix         map[string][]yaml.Node

	flatMatrix    []map[string]yaml.Node
	includeMatrix []map[string]yaml.Node

	include []yaml.Node
	exclude []yaml.Node
}

func (strategyContext *strategyContext) handleInclude() error {
	// Handle include logic
	if len(strategyContext.include) > 0 {
		for _, incNode := range strategyContext.include {
			if incNode.Kind != yaml.MappingNode {
				return fmt.Errorf("include entry is not a mapping node")
			}
			incMap := make(map[string]yaml.Node)
			for i := 0; i < len(incNode.Content); i += 2 {
				keyNode := incNode.Content[i]
				valNode := incNode.Content[i+1]
				if keyNode.Kind != yaml.ScalarNode {
					return fmt.Errorf("include key is not scalar")
				}
				incMap[keyNode.Value] = *valNode
			}
			matched := false
			for _, row := range strategyContext.flatMatrix {
				match := true
				for k, v := range incMap {
					if rv, ok := row[k]; ok && !nodesEqual(rv, v) {
						match = false
						break
					}
				}
				if match {
					matched = true
					// Add missing keys
					strategyContext.jobTraceWriter.Info("Add missing keys %v", incMap)
					for k, v := range incMap {
						if _, ok := row[k]; !ok {
							row[k] = v
						}
					}
				}
			}
			if !matched {
				if strategyContext.jobTraceWriter != nil {
					strategyContext.jobTraceWriter.Info("Append include entry %v", incMap)
				}
				strategyContext.includeMatrix = append(strategyContext.includeMatrix, incMap)
			}
		}
	}
	return nil
}

func (strategyContext *strategyContext) handleExclude() error {
	// Handle exclude logic
	if len(strategyContext.exclude) > 0 {
		for _, exNode := range strategyContext.exclude {
			// exNode is expected to be a mapping node
			if exNode.Kind != yaml.MappingNode {
				return fmt.Errorf("exclude entry is not a mapping node")
			}
			// Convert mapping to map[string]yaml.Node
			exMap := make(map[string]yaml.Node)
			for i := 0; i < len(exNode.Content); i += 2 {
				keyNode := exNode.Content[i]
				valNode := exNode.Content[i+1]
				if keyNode.Kind != yaml.ScalarNode {
					return fmt.Errorf("exclude key is not scalar")
				}
				exMap[keyNode.Value] = *valNode
			}
			// Remove matching rows
			filtered := []map[string]yaml.Node{}
			for _, row := range strategyContext.flatMatrix {
				match := true
				for k, v := range exMap {
					if rv, ok := row[k]; !ok || !nodesEqual(rv, v) {
						match = false
						break
					}
				}
				if !match {
					filtered = append(filtered, row)
				} else if strategyContext.jobTraceWriter != nil {
					strategyContext.jobTraceWriter.Info("Removing %v from matrix due to exclude entry %v", row, exMap)
				}
			}
			strategyContext.flatMatrix = filtered
		}
	}
	return nil
}

// ExpandStrategy expands the given strategy into a flat matrix and include matrix.
// It mimics the behavior of the C# StrategyUtils. The strategy parameter is expected
// to be populated from a YAML mapping that follows the GitHub Actions strategy schema.
func ExpandStrategy(strategy *Strategy, jobTraceWriter TraceWriter) (*StrategyResult, error) {
	if strategy == nil {
		return &StrategyResult{FlatMatrix: []map[string]yaml.Node{{}}, IncludeMatrix: []map[string]yaml.Node{}, FailFast: true}, nil
	}

	// Initialize defaults
	strategyContext := &strategyContext{
		jobTraceWriter: jobTraceWriter,
		failFast:       strategy.FailFast,
		maxParallel:    strategy.MaxParallel,
		matrix:         strategy.Matrix,
		flatMatrix:     []map[string]yaml.Node{{}},
	}
	// Process matrix entries
	for key, values := range strategyContext.matrix {
		switch key {
		case "include":
			strategyContext.include = values
		case "exclude":
			strategyContext.exclude = values
		default:
			// Other keys are treated as matrix dimensions
			// Expand each existing row with the new key/value pairs
			next := []map[string]yaml.Node{}
			for _, row := range strategyContext.flatMatrix {
				for _, val := range values {
					newRow := make(map[string]yaml.Node)
					for k, v := range row {
						newRow[k] = v
					}
					newRow[key] = val
					next = append(next, newRow)
				}
			}
			strategyContext.flatMatrix = next
		}
	}

	if err := strategyContext.handleExclude(); err != nil {
		return nil, err
	}

	if len(strategyContext.flatMatrix) == 0 {
		if jobTraceWriter != nil {
			jobTraceWriter.Info("Matrix is empty, adding an empty entry")
		}
		strategyContext.flatMatrix = []map[string]yaml.Node{{}}
	}

	// Enforce job matrix limit of github
	if len(strategyContext.flatMatrix) > 256 {
		if jobTraceWriter != nil {
			jobTraceWriter.Info("Failure: Matrix contains more than 256 entries after exclude")
		}
		return nil, errors.New("matrix contains more than 256 entries")
	}

	// Build matrix keys set
	matrixKeys := make(map[string]struct{})
	if len(strategyContext.flatMatrix) > 0 {
		for k := range strategyContext.flatMatrix[0] {
			matrixKeys[k] = struct{}{}
		}
	}

	if err := strategyContext.handleInclude(); err != nil {
		return nil, err
	}

	return &StrategyResult{
		FlatMatrix:    strategyContext.flatMatrix,
		IncludeMatrix: strategyContext.includeMatrix,
		FailFast:      strategyContext.failFast,
		MaxParallel:   &strategyContext.maxParallel,
		MatrixKeys:    matrixKeys,
	}, nil
}

// nodesEqual compares two yaml.Node values for equality.
func nodesEqual(a, b yaml.Node) bool {
	return DeepEquals(a, b, true)
}

// GetDefaultDisplaySuffix returns a string like "(foo, bar, baz)".
// Empty items are ignored. If all items are empty the result is "".
func GetDefaultDisplaySuffix(items []string) string {
	var b strings.Builder // efficient string concatenation

	first := true // true until we write the first non‑empty item

	for _, mk := range items {
		if mk == "" { // Go has no null string, so we only need to check for empty
			continue
		}
		if first {
			b.WriteString("(")
			first = false
		} else {
			b.WriteString(", ")
		}
		b.WriteString(mk)
	}

	if !first { // we wrote at least one item
		b.WriteString(")")
	}

	return b.String()
}
