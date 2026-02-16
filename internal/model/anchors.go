package model

import (
	"errors"

	"go.yaml.in/yaml/v4"
)

// Assumes there is no cycle ensured via test TestVerifyCycleIsInvalid
func resolveAliases(node *yaml.Node) error {
	switch node.Kind {
	case yaml.AliasNode:
		aliasTarget := node.Alias
		if aliasTarget == nil {
			return errors.New("unresolved alias node")
		}
		*node = *aliasTarget
		if err := resolveAliases(node); err != nil {
			return err
		}

	case yaml.DocumentNode, yaml.MappingNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := resolveAliases(child); err != nil {
				return err
			}
		}
	}
	return nil
}
