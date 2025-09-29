package model

import (
	"strings"

	v2 "github.com/actions-oss/act-cli/internal/eval/v2"
	"gopkg.in/yaml.v3"
)

// DeepEquals compares two yaml.Node values recursively.
// It supports scalar, mapping and sequence nodes and allows
// an optional partial match for mappings and sequences.
func DeepEquals(a, b yaml.Node, partialMatch bool) bool {
	// Scalar comparison
	if a.Kind == yaml.ScalarNode && b.Kind == yaml.ScalarNode {
		return scalarEquals(a, b)
	}

	// Mapping comparison
	if a.Kind == yaml.MappingNode && b.Kind == yaml.MappingNode {
		return deepMapEquals(a, b, partialMatch)
	}

	// Sequence comparison
	if a.Kind == yaml.SequenceNode && b.Kind == yaml.SequenceNode {
		return deepSequenceEquals(a, b, partialMatch)
	}

	// Different kinds are not equal
	return false
}

func scalarEquals(a, b yaml.Node) bool {
	var left, right any
	return a.Decode(&left) == nil && b.Decode(&right) == nil && v2.CreateIntermediateResult(v2.NewEvaluationContext(), left).AbstractEqual(v2.CreateIntermediateResult(v2.NewEvaluationContext(), right))
}

func deepMapEquals(a, b yaml.Node, partialMatch bool) bool {
	mapA := make(map[string]yaml.Node)
	for i := 0; i < len(a.Content); i += 2 {
		keyNode := a.Content[i]
		valNode := a.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return false
		}
		mapA[strings.ToLower(keyNode.Value)] = *valNode
	}
	mapB := make(map[string]yaml.Node)
	for i := 0; i < len(b.Content); i += 2 {
		keyNode := b.Content[i]
		valNode := b.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return false
		}
		mapB[strings.ToLower(keyNode.Value)] = *valNode
	}
	if partialMatch {
		if len(mapA) < len(mapB) {
			return false
		}
	} else {
		if len(mapA) != len(mapB) {
			return false
		}
	}
	for k, vB := range mapB {
		vA, ok := mapA[k]
		if !ok || !DeepEquals(vA, vB, partialMatch) {
			return false
		}
	}
	return true
}

func deepSequenceEquals(a, b yaml.Node, partialMatch bool) bool {
	if partialMatch {
		if len(a.Content) < len(b.Content) {
			return false
		}
	} else {
		if len(a.Content) != len(b.Content) {
			return false
		}
	}
	limit := len(b.Content)
	if !partialMatch {
		limit = len(a.Content)
	}
	for i := 0; i < limit; i++ {
		if !DeepEquals(*a.Content[i], *b.Content[i], partialMatch) {
			return false
		}
	}
	return true
}

// traverse walks a YAML node recursively.
func traverse(node *yaml.Node, omitKeys bool, result *[]*yaml.Node) {
	if node == nil {
		return
	}

	*result = append(*result, node)

	switch node.Kind {
	case yaml.MappingNode:
		if omitKeys {
			// node.Content: key0, val0, key1, val1, …
			for i := 1; i < len(node.Content); i += 2 { // only the values
				traverse(node.Content[i], omitKeys, result)
			}
		} else {
			for _, child := range node.Content {
				traverse(child, omitKeys, result)
			}
		}
	case yaml.SequenceNode:
		// For all other node kinds (Scalar, Sequence, Alias, etc.)
		for _, child := range node.Content {
			traverse(child, omitKeys, result)
		}
	}
}

// GetDisplayStrings implements the LINQ expression:
//
//	from displayitem in keys.SelectMany(key => item[key].Traverse(true))
//	where !(displayitem is SequenceToken || displayitem is MappingToken)
//	select displayitem.ToString()
func GetDisplayStrings(keys []string, item map[string]*yaml.Node) []string {
	var res []string

	for _, k := range keys {
		if node, ok := item[k]; ok {
			var all []*yaml.Node
			traverse(node, true, &all) // include the parent node itself

			for _, n := range all {
				// Keep only scalars – everything else is dropped
				if n.Kind == yaml.ScalarNode {
					res = append(res, n.Value)
				}
			}
		}
	}

	return res
}
