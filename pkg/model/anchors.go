package model

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func resolveAliasesExt(node *yaml.Node, path map[*yaml.Node]bool) error {
	switch node.Kind {
	case yaml.AliasNode:
		aliasTarget := node.Alias
		if aliasTarget == nil {
			return errors.New("unresolved alias node")
		}
		if path[aliasTarget] {
			return errors.New("regression detected: circular alias")
		}
		path[aliasTarget] = true
		*node = *aliasTarget
		if err := resolveAliasesExt(node, path); err != nil {
			return err
		}
		delete(path, aliasTarget)

	case yaml.MappingNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := resolveAliasesExt(child, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveAliases(node *yaml.Node) error {
	return resolveAliasesExt(node, map[*yaml.Node]bool{})
}
