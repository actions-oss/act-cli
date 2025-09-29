package v2

import (
	"testing"
)

// Test boolean and comparison operations using the evaluator.
func TestEvaluator_BooleanOps(t *testing.T) {
	ctx := &EvaluationContext{Variables: CaseInsensitiveObject[any](map[string]interface{}{"a": 5, "b": 3})}
	eval := NewEvaluator(ctx)

	tests := []struct {
		expr string
		want bool
	}{
		{"1 == 1", true},
		{"1 != 2", true},
		{"5 > 3", true},
		{"2 < 4", true},
		{"5 >= 5", true},
		{"3 <= 4", true},
		{"true && false", false},
		{"!false", true},
		{"a > b", true},
	}

	for _, tt := range tests {
		got, err := eval.EvaluateBoolean(tt.expr)
		if err != nil {
			t.Fatalf("evaluate %s error: %v", tt.expr, err)
		}
		if got != tt.want {
			t.Fatalf("evaluate %s expected %v got %v", tt.expr, tt.want, got)
		}
	}
}

func TestEvaluator_Raw(t *testing.T) {
	ctx := &EvaluationContext{
		Variables: CaseInsensitiveObject[any](map[string]any{"a": 5, "b": 3}),
		Functions: GetFunctions(),
	}
	eval := NewEvaluator(ctx)

	tests := []struct {
		expr string
		want interface{}
	}{
		{"a.b['x']", nil},
		{"(a.b).c['x']", nil},
		{"(a.b).*['x']", nil},
		{"(a['x'])", nil},
		{"true || false", true},
		{"false || false", false},
		{"false || true", true},
		{"false || true || false", true},
		{"contains('', '') || contains('', '') || contains('', '')", true},
		{"1 == 1", true},
		{"1 != 2", true},
		{"5 > 3", true},
		{"2 < 4", true},
		{"5 >= 5", true},
		{"3 <= 4", true},
		{"true && false", false},
		{"!false", true},
		{"a > b", true},
		{"!(a > b)", false},
		{"!(a > b) || !0", true},
		{"!(a > b) || !(1)", false},
		{"'Hello World'", "Hello World"},
		{"23.5", 23.5},
		{"fromjson('{\"twst\":\"x\"}')['twst']", "x"},
		{"fromjson('{\"Twst\":\"x\"}')['twst']", "x"},
		{"fromjson('{\"TwsT\":\"x\"}')['twst']", "x"},
		{"fromjson('{\"TwsT\":\"x\"}')['tWst']", "x"},
		{"fromjson('{\"TwsT\":{\"a\":\"y\"}}').TwsT.a", "y"},
		{"fromjson('{\"TwsT\":{\"a\":\"y\"}}')['TwsT'].a", "y"},
		{"fromjson('{\"TwsT\":{\"a\":\"y\"}}')['TwsT']['a']", "y"},
		{"fromjson('{\"TwsT\":{\"a\":\"y\"}}').TwsT['a']", "y"},
		// {"fromjson('{\"TwsT\":\"x\"}').*[0]", "x"},
		{"fromjson('{\"TwsT\":[\"x\"]}')['TwsT'][0]", "x"},
		{"fromjson('[]')['tWst']", nil},
		{"fromjson('[]').tWst", nil},
		{"contains('a', 'a')", true},
		{"contains('bab', 'a')", true},
		{"contains('bab', 'ac')", false},
		{"contains(fromjson('[\"ac\"]'), 'ac')", true},
		{"contains(fromjson('[\"ac\"]'), 'a')", false},
		// {"fromjson('{\"TwsT\":{\"a\":\"y\"}}').*['a']", "y"},
		{"fromjson(tojson(fromjson('{\"TwsT\":{\"a\":\"y\"}}').*.a))[0]", "y"},
		{"fromjson(tojson(fromjson('{\"TwsT\":{\"a\":\"y\"}}').*['a']))[0]", "y"},
		{"fromjson('{}').x", nil},
		{"format('{0}', fromjson('{}').x)", ""},
		{"format('{0}', fromjson('{}')[0])", ""},
		{"fromjson(tojson(fromjson('[[3,5],[5,6]]').*[1]))[1]", float64(6)},
		{"contains(fromjson('[[3,5],[5,6]]').*[1], 5)", true},
		{"contains(fromjson('[[3,5],[5,6]]').*[1], 6)", true},
		{"contains(fromjson('[[3,5],[5,6]]').*[1], 3)", false},
		{"contains(fromjson('[[3,5],[5,6]]').*[1], '6')", true},
	}

	for _, tt := range tests {
		got, err := eval.EvaluateRaw(tt.expr)
		if err != nil {
			t.Fatalf("evaluate %s error: %v", tt.expr, err)
		}
		if got != tt.want {
			t.Fatalf("evaluate %s expected %v got %v", tt.expr, tt.want, got)
		}
	}
}
