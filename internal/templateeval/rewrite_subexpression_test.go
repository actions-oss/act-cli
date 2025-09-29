package templateeval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteSubExpression_NoExpression(t *testing.T) {
	in := "Hello world"
	out, ok, err := rewriteSubExpression(in, false)
	assert.NoError(t, err)
	if ok {
		t.Fatalf("expected ok=false for no expression, got true with output %q", out)
	}
	if out != in {
		t.Fatalf("expected output %q, got %q", in, out)
	}
}

func TestRewriteSubExpression_SingleExpression(t *testing.T) {
	in := "Hello ${{ 'world' }}"
	out, ok, err := rewriteSubExpression(in, false)
	assert.NoError(t, err)
	if !ok {
		t.Fatalf("expected ok=true for single expression, got false")
	}
	expected := "format('Hello {0}', 'world')"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestRewriteSubExpression_MultipleExpressions(t *testing.T) {
	in := "Hello ${{ 'world' }}, you are ${{ 'awesome' }}"
	out, ok, err := rewriteSubExpression(in, false)
	assert.NoError(t, err)
	if !ok {
		t.Fatalf("expected ok=true for multiple expressions, got false")
	}
	expected := "format('Hello {0}, you are {1}', 'world', 'awesome')"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestRewriteSubExpression_ForceFormatSingle(t *testing.T) {
	in := "Hello ${{ 'world' }}"
	out, ok, err := rewriteSubExpression(in, true)
	assert.NoError(t, err)
	if !ok {
		t.Fatalf("expected ok=true when forceFormat, got false")
	}
	expected := "format('Hello {0}', 'world')"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestRewriteSubExpression_ForceFormatMultiple(t *testing.T) {
	in := "Hello ${{ 'world' }}, you are ${{ 'awesome' }}"
	out, ok, err := rewriteSubExpression(in, true)
	assert.NoError(t, err)
	if !ok {
		t.Fatalf("expected ok=true when forceFormat, got false")
	}
	expected := "format('Hello {0}, you are {1}', 'world', 'awesome')"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestRewriteSubExpression_UnclosedExpression(t *testing.T) {
	in := "Hello ${{ 'world' " // missing closing }}
	_, _, err := rewriteSubExpression(in, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed expression")
}

func TestRewriteSubExpression_UnclosedString(t *testing.T) {
	in := "Hello ${{ 'world }}, you are ${{ 'awesome' }}"
	_, _, err := rewriteSubExpression(in, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed string")
}

func TestRewriteSubExpression_EscapedStringLiteral(t *testing.T) {
	// Two single quotes represent an escaped quote inside a string
	in := "Hello ${{ 'It''s a test' }}"
	out, ok, err := rewriteSubExpression(in, false)
	assert.NoError(t, err)
	assert.True(t, ok)
	expected := "format('Hello {0}', 'It''s a test')"
	assert.Equal(t, expected, out)
}

func TestRewriteSubExpression_ExpressionAtEnd(t *testing.T) {
	// Expression ends exactly at the string end – should be valid
	in := "Hello ${{ 'world' }}"
	out, ok, err := rewriteSubExpression(in, false)
	assert.NoError(t, err)
	assert.True(t, ok)
	expected := "format('Hello {0}', 'world')"
	assert.Equal(t, expected, out)
}

func TestRewriteSubExpression_ExpressionNotAtEnd(t *testing.T) {
	// Expression followed by additional text – should still be valid
	in := "Hello ${{ 'world' }}, how are you?"
	out, ok, err := rewriteSubExpression(in, false)
	assert.NoError(t, err)
	assert.True(t, ok)
	expected := "format('Hello {0}, how are you?', 'world')"
	assert.Equal(t, expected, out)
}
