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
