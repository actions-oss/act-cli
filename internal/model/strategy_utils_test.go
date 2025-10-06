package model

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type EmptyTraceWriter struct {
}

func (e *EmptyTraceWriter) Info(_ string, _ ...interface{}) {
}

func TestStrategy(t *testing.T) {
	table := []struct {
		content       string
		flatmatrix    int
		includematrix int
	}{
		{`
matrix:
  label:
  - a
  - b
  fields:
  - a
  - b
`, 4, 0},
		{`
matrix:
  label:
  - a
  - b
  include:
  - label: a
    x: self`, 2, 0,
		},
		{`
matrix:
  label:
  - a
  - b
  include:
  - label: c
    x: self`, 2, 1,
		},
		{`
matrix:
  label:
  - a
  - b
  exclude:
  - label: a`, 1, 0,
		},
	}

	for _, tc := range table {
		var strategy Strategy
		err := yaml.Unmarshal([]byte(tc.content), &strategy)
		require.NoError(t, err)
		res, err := ExpandStrategy(&strategy, &EmptyTraceWriter{})
		require.NoError(t, err)
		require.Len(t, res.FlatMatrix, tc.flatmatrix)
		require.Len(t, res.IncludeMatrix, tc.includematrix)
	}
}
