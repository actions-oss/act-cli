package workflow

import "testing"

func TestExpressionParser(t *testing.T) {
	node, err := Parse("github.event_name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Parsed expression: %+v", node)
}

func TestExpressionParserWildcard(t *testing.T) {
	node, err := Parse("github.commits.*.message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Parsed expression: %+v", node)
}

func TestExpressionParserDot(t *testing.T) {
	node, err := Parse("github.head_commit.message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Parsed expression: %+v", node)
}
