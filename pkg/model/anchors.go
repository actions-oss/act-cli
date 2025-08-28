package model

import (
	"gopkg.in/yaml.v3"
)

func resolveAliases(node *yaml.Node, anchors map[string]*yaml.Node, path map[*yaml.Node]bool) error {
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
        if err := resolveAliases(node, anchors, path); err != nil {
            return err
        }
        delete(path, aliasTarget)

    case yaml.MappingNode, yaml.SequenceNode:
        for _, child := range node.Content {
            if err := resolveAliases(child, anchors, path); err != nil {
                return err
            }
        }
    }
    return nil
}
