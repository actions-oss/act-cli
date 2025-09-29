package workflow

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexer(t *testing.T) {
	input := "github.event_name == 'push' && github.ref == 'refs/heads/main'"
	lexer := NewLexer(input, 0)
	var tokens []*Token
	for {
		tok := lexer.Next()
		if tok == nil || tok.Kind == TokenKindUnexpected {
			break
		}
		tokens = append(tokens, tok)
	}
	for i, tok := range tokens {
		t.Logf("Token %d: Kind=%v, Value=%v", i, tok.Kind, tok.Value)
	}
	assert.Equal(t, tokens[1].Kind, TokenKindDereference)
}

func TestLexerNumbers(t *testing.T) {
	table := []struct {
		in  string
		out interface{}
	}{
		{"-Infinity", math.Inf(-1)},
		{"Infinity", math.Inf(1)},
		{"2.5", float64(2.5)},
		{"3.3", float64(3.3)},
		{"1", float64(1)},
		{"-1", float64(-1)},
		{"0x34", float64(0x34)},
		{"0o34", float64(0o34)},
	}
	for _, cs := range table {
		lexer := NewLexer(cs.in, 0)
		var tokens []*Token
		for {
			tok := lexer.Next()
			if tok == nil || tok.Kind == TokenKindUnexpected {
				break
			}
			tokens = append(tokens, tok)
		}
		require.Len(t, tokens, 1)
		assert.Equal(t, cs.out, tokens[0].Value)
		assert.Equal(t, cs.in, tokens[0].Raw)
	}
}
