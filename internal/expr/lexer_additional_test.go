package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLexerMultiple runs a set of expressions through the lexer and
// verifies that the produced token kinds and values match expectations.
func TestLexerMultiple(t *testing.T) {
	cases := []struct {
		expr     string
		expected []TokenKind
		values   []interface{} // optional, nil if not checking values
	}{
		{
			expr: "github.event_name == 'push'",
			expected: []TokenKind{
				TokenKindNamedValue, // github
				TokenKindDereference,
				TokenKindPropertyName,    // event_name
				TokenKindLogicalOperator, // ==
				TokenKindString,          // 'push'
			},
		},
		{
			expr: "github.event_name == 'push' && github.ref == 'refs/heads/main'",
			expected: []TokenKind{
				TokenKindNamedValue, TokenKindDereference, TokenKindPropertyName, TokenKindLogicalOperator, TokenKindString,
				TokenKindLogicalOperator, // &&
				TokenKindNamedValue, TokenKindDereference, TokenKindPropertyName, TokenKindLogicalOperator, TokenKindString,
			},
		},
		{
			expr: "contains(github.ref, 'refs/heads/')",
			expected: []TokenKind{
				TokenKindFunction, // contains
				TokenKindStartParameters,
				TokenKindNamedValue, TokenKindDereference, TokenKindPropertyName, // github.ref
				TokenKindSeparator,
				TokenKindString,
				TokenKindEndParameters,
			},
		},
		{
			expr: "matrix[0].name",
			expected: []TokenKind{
				TokenKindNamedValue, // matrix
				TokenKindStartIndex,
				TokenKindNumber,
				TokenKindEndIndex,
				TokenKindDereference,
				TokenKindPropertyName, // name
			},
		},
		{
			expr: "github.*",
			expected: []TokenKind{
				TokenKindNamedValue, TokenKindDereference, TokenKindWildcard,
			},
		},
		{
			expr:     "null",
			expected: []TokenKind{TokenKindNull},
		},
		{
			expr:     "true",
			expected: []TokenKind{TokenKindBoolean},
			values:   []interface{}{true},
		},
		{
			expr:     "123",
			expected: []TokenKind{TokenKindNumber},
			values:   []interface{}{123.0},
		},
		{
			expr:     "(a && b)",
			expected: []TokenKind{TokenKindStartGroup, TokenKindNamedValue, TokenKindLogicalOperator, TokenKindNamedValue, TokenKindEndGroup},
		},
		{
			expr:     "[1,2]", // Syntax Error
			expected: []TokenKind{TokenKindUnexpected, TokenKindNumber, TokenKindSeparator, TokenKindNumber, TokenKindEndIndex},
		},
		{
			expr:     "'Hello i''s escaped'",
			expected: []TokenKind{TokenKindString},
			values:   []interface{}{"Hello i's escaped"},
		},
	}

	for _, tc := range cases {
		lexer := NewLexer(tc.expr, 0)
		var tokens []*Token
		for {
			tok := lexer.Next()
			if tok == nil {
				break
			}
			tokens = append(tokens, tok)
		}
		assert.Equal(t, len(tc.expected), len(tokens), "expression: %s", tc.expr)
		for i, kind := range tc.expected {
			assert.Equal(t, kind, tokens[i].Kind, "expr %s token %d", tc.expr, i)
		}
		if tc.values != nil {
			for i, val := range tc.values {
				assert.Equal(t, val, tokens[i].Value, "expr %s token %d value", tc.expr, i)
			}
		}
	}
}
