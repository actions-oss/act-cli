package model

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
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

// ExpandStrategy expands the given strategy into a flat matrix and include matrix.
// It mimics the behavior of the C# StrategyUtils. The strategy parameter is expected
// to be populated from a YAML mapping that follows the GitHub Actions strategy schema.
func ExpandStrategy(strategy *Strategy, matrixExcludeIncludeLists bool, jobTraceWriter TraceWriter, jobName string) (*StrategyResult, error) {
	if strategy == nil {
		return &StrategyResult{FlatMatrix: []map[string]yaml.Node{{}}, IncludeMatrix: []map[string]yaml.Node{}, FailFast: true}, nil
	}

	// Initialize defaults
	failFast := strategy.FailFast
	maxParallel := strategy.MaxParallel
	matrix := strategy.Matrix

	// Prepare containers
	flatMatrix := []map[string]yaml.Node{{}}
	includeMatrix := []map[string]yaml.Node{}

	// Process matrix entries
	var include []yaml.Node
	var exclude []yaml.Node
	for key, values := range matrix {
		switch key {
		case "include":
			include = values
		case "exclude":
			exclude = values
		default:
			// Other keys are treated as matrix dimensions
			// Expand each existing row with the new key/value pairs
			next := []map[string]yaml.Node{}
			for _, row := range flatMatrix {
				for _, val := range values {
					newRow := make(map[string]yaml.Node)
					for k, v := range row {
						newRow[k] = v
					}
					newRow[key] = val
					next = append(next, newRow)
				}
			}
			flatMatrix = next
		}
	}

	// Handle exclude logic
	if len(exclude) > 0 {
		for _, exNode := range exclude {
			// exNode is expected to be a mapping node
			if exNode.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("exclude entry is not a mapping node")
			}
			// Convert mapping to map[string]yaml.Node
			exMap := make(map[string]yaml.Node)
			for i := 0; i < len(exNode.Content); i += 2 {
				keyNode := exNode.Content[i]
				valNode := exNode.Content[i+1]
				if keyNode.Kind != yaml.ScalarNode {
					return nil, fmt.Errorf("exclude key is not scalar")
				}
				exMap[keyNode.Value] = *valNode
			}
			// Remove matching rows
			filtered := []map[string]yaml.Node{}
			for _, row := range flatMatrix {
				match := true
				for k, v := range exMap {
					if rv, ok := row[k]; !ok || !nodesEqual(rv, v) {
						match = false
						break
					}
				}
				if !match {
					filtered = append(filtered, row)
				} else if jobTraceWriter != nil {
					jobTraceWriter.Info("Removing %v from matrix due to exclude entry %v", row, exMap)
				}
			}
			flatMatrix = filtered
		}
	}

	if len(flatMatrix) == 0 {
		if jobTraceWriter != nil {
			jobTraceWriter.Info("Matrix is empty, adding an empty entry")
		}
		flatMatrix = []map[string]yaml.Node{{}}
	}

	// Enforce job matrix limit of github
	if len(flatMatrix) > 256 {
		if jobTraceWriter != nil {
			jobTraceWriter.Info("Failure: Matrix contains more than 256 entries after exclude")
		}
		return nil, errors.New("matrix contains more than 256 entries")
	}

	// Build matrix keys set
	matrixKeys := make(map[string]struct{})
	if len(flatMatrix) > 0 {
		for k := range flatMatrix[0] {
			matrixKeys[k] = struct{}{}
		}
	}

	// Handle include logic
	if len(include) > 0 {
		for _, incNode := range include {
			if incNode.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("include entry is not a mapping node")
			}
			incMap := make(map[string]yaml.Node)
			for i := 0; i < len(incNode.Content); i += 2 {
				keyNode := incNode.Content[i]
				valNode := incNode.Content[i+1]
				if keyNode.Kind != yaml.ScalarNode {
					return nil, fmt.Errorf("include key is not scalar")
				}
				incMap[keyNode.Value] = *valNode
			}
			matched := false
			for _, row := range flatMatrix {
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
					jobTraceWriter.Info("Add missing keys %v", incMap)
					for k, v := range incMap {
						if _, ok := row[k]; !ok {
							row[k] = v
						}
					}
				}
			}
			if !matched {
				if jobTraceWriter != nil {
					jobTraceWriter.Info("Append include entry %v", incMap)
				}
				includeMatrix = append(includeMatrix, incMap)
			}
		}
	}

	return &StrategyResult{
		FlatMatrix:    flatMatrix,
		IncludeMatrix: includeMatrix,
		FailFast:      failFast,
		MaxParallel:   &maxParallel,
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
