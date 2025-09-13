package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestVerifyCycleIsInvalid(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
a: &a
  ref: *b

b: &b
  ref: *a
`), &node)
	assert.Error(t, err)
}

func TestVerifyNilAliasError(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
test:
- a
- b
- c`), &node)
	*node.Content[0].Content[1].Content[1] = yaml.Node{
		Kind: yaml.AliasNode,
	}
	assert.NoError(t, err)
	err = resolveAliases(&node)
	assert.Error(t, err)
}
