package v2

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ValueKind represents the type of a value in the evaluation engine.
// The values mirror the C# ValueKind enum.
//
// Note: The names are kept identical to the C# implementation for easier mapping.
//
// The lexer is intentionally simple – it only tokenises the subset of
// expressions that are used in GitHub Actions workflow `if:` expressions.
// It does not evaluate the expression – that is left to the parser.

type ValueKind int

const (
	ValueKindNull ValueKind = iota
	ValueKindBoolean
	ValueKindNumber
	ValueKindString
	ValueKindObject
	ValueKindArray
)

type ReadOnlyArray[T any] interface {
	GetAt(i int64) T
	GetEnumerator() []T
}

type ReadOnlyObject[T any] interface {
	Get(key string) T
	GetEnumerator() map[string]T
}

type BasicArray[T any] []T

func (a BasicArray[T]) GetAt(i int64) T {
	if int(i) >= len(a) {
		var zero T
		return zero
	}
	return a[i]
}

func (a BasicArray[T]) GetEnumerator() []T {
	return a
}

type CaseInsensitiveObject[T any] map[string]T

func (o CaseInsensitiveObject[T]) Get(key string) T {
	for k, v := range o {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	var zero T
	return zero
}

func (o CaseInsensitiveObject[T]) GetEnumerator() map[string]T {
	return o
}

type CaseSensitiveObject[T any] map[string]T

func (o CaseSensitiveObject[T]) Get(key string) T {
	return o[key]
}

func (o CaseSensitiveObject[T]) GetEnumerator() map[string]T {
	return o
}

// EvaluationResult holds the result of evaluating an expression node.
// It mirrors the C# EvaluationResult class.

type EvaluationResult struct {
	context     *EvaluationContext
	level       int
	value       interface{}
	kind        ValueKind
	raw         interface{}
	omitTracing bool
}

// NewEvaluationResult creates a new EvaluationResult.
func NewEvaluationResult(context *EvaluationContext, level int, val interface{}, kind ValueKind, raw interface{}, omitTracing bool) *EvaluationResult {
	er := &EvaluationResult{context: context, level: level, value: val, kind: kind, raw: raw, omitTracing: omitTracing}
	if !omitTracing {
		er.traceValue()
	}
	return er
}

// Kind returns the ValueKind of the result.
func (er *EvaluationResult) Kind() ValueKind { return er.kind }

// Raw returns the raw value that was passed to the constructor.
func (er *EvaluationResult) Raw() interface{} { return er.raw }

// Value returns the canonical value.
func (er *EvaluationResult) Value() interface{} { return er.value }

// IsFalsy implements the logic from the C# class.
func (er *EvaluationResult) IsFalsy() bool {
	switch er.kind {
	case ValueKindNull:
		return true
	case ValueKindBoolean:
		return !er.value.(bool)
	case ValueKindNumber:
		v := er.value.(float64)
		return v == 0 || isNaN(v)
	case ValueKindString:
		return er.value.(string) == ""
	default:
		return false
	}
}

func isNaN(v float64) bool { return v != v }

// IsPrimitive returns true if the kind is a primitive type.
func (er *EvaluationResult) IsPrimitive() bool { return er.kind <= ValueKindString }

// IsTruthy is the negation of IsFalsy.
func (er *EvaluationResult) IsTruthy() bool { return !er.IsFalsy() }

// AbstractEqual compares two EvaluationResults using the abstract equality algorithm.
func (er *EvaluationResult) AbstractEqual(other *EvaluationResult) bool {
	return abstractEqual(er.value, other.value)
}

// AbstractGreaterThan compares two EvaluationResults.
func (er *EvaluationResult) AbstractGreaterThan(other *EvaluationResult) bool {
	return abstractGreaterThan(er.value, other.value)
}

// AbstractGreaterThanOrEqual
func (er *EvaluationResult) AbstractGreaterThanOrEqual(other *EvaluationResult) bool {
	return er.AbstractEqual(other) || er.AbstractGreaterThan(other)
}

// AbstractLessThan
func (er *EvaluationResult) AbstractLessThan(other *EvaluationResult) bool {
	return abstractLessThan(er.value, other.value)
}

// AbstractLessThanOrEqual
func (er *EvaluationResult) AbstractLessThanOrEqual(other *EvaluationResult) bool {
	return er.AbstractEqual(other) || er.AbstractLessThan(other)
}

// AbstractNotEqual
func (er *EvaluationResult) AbstractNotEqual(other *EvaluationResult) bool {
	return !er.AbstractEqual(other)
}

// ConvertToNumber converts the value to a float64.
func (er *EvaluationResult) ConvertToNumber() float64 { return convertToNumber(er.value) }

// ConvertToString converts the value to a string.
func (er *EvaluationResult) ConvertToString() string {
	switch er.kind {
	case ValueKindNull:
		return ""
	case ValueKindBoolean:
		if er.value.(bool) {
			return ExpressionConstants.True
		}
		return ExpressionConstants.False
	case ValueKindNumber:
		return fmt.Sprintf(ExpressionConstants.NumberFormat, er.value.(float64))
	case ValueKindString:
		return er.value.(string)
	default:
		return fmt.Sprintf("%v", er.value)
	}
}

// TryGetCollectionInterface returns the underlying collection if the value is an array or object.
func (er *EvaluationResult) TryGetCollectionInterface() (interface{}, bool) {
	switch v := er.value.(type) {
	case ReadOnlyArray[any]:
		return v, true
	case ReadOnlyObject[any]:
		return v, true
	default:
		return nil, false
	}
}

// CreateIntermediateResult creates an EvaluationResult from an arbitrary object.
func CreateIntermediateResult(context *EvaluationContext, obj interface{}) *EvaluationResult {
	val, kind, raw := convertToCanonicalValue(obj)
	return NewEvaluationResult(context, 0, val, kind, raw, true)
}

// --- Helper functions and constants ---------------------------------------

// ExpressionConstants holds string constants used in conversions.
var ExpressionConstants = struct {
	True         string
	False        string
	NumberFormat string
}{
	True:         "true",
	False:        "false",
	NumberFormat: "%.15g",
}

// convertToCanonicalValue converts an arbitrary Go value to a canonical form.
func convertToCanonicalValue(obj interface{}) (interface{}, ValueKind, interface{}) {
	switch v := obj.(type) {
	case nil:
		return nil, ValueKindNull, nil
	case bool:
		return v, ValueKindBoolean, v
	case int, int8, int16, int32, int64:
		f := float64(toInt64(v))
		return f, ValueKindNumber, f
	case uint, uint8, uint16, uint32, uint64:
		f := float64(toUint64(v))
		return f, ValueKindNumber, f
	case float32, float64:
		f := toFloat64(v)
		return f, ValueKindNumber, f
	case string:
		return v, ValueKindString, v
	case []interface{}:
		return BasicArray[any](v), ValueKindArray, v
	case ReadOnlyArray[any]:
		return v, ValueKindArray, v
	case map[string]interface{}:
		return CaseInsensitiveObject[any](v), ValueKindObject, v
	case ReadOnlyObject[any]:
		return v, ValueKindObject, v
	default:
		// Fallback: treat as object
		return v, ValueKindObject, v
	}
}

func toInt64(v interface{}) int64 {
	switch i := v.(type) {
	case int:
		return int64(i)
	case int8:
		return int64(i)
	case int16:
		return int64(i)
	case int32:
		return int64(i)
	case int64:
		return i
	default:
		return 0
	}
}

func toUint64(v interface{}) uint64 {
	switch i := v.(type) {
	case uint:
		return uint64(i)
	case uint8:
		return uint64(i)
	case uint16:
		return uint64(i)
	case uint32:
		return uint64(i)
	case uint64:
		return i
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch f := v.(type) {
	case float32:
		return float64(f)
	case float64:
		return f
	default:
		return 0
	}
}

// coerceTypes implements the C# CoerceTypes logic.
// It converts values to compatible types before comparison.
func coerceTypes(left, right interface{}) (interface{}, interface{}, ValueKind, ValueKind) {
	leftKind := getKind(left)
	rightKind := getKind(right)

	// same kind – nothing to do
	if leftKind == rightKind {
		return left, right, leftKind, rightKind
	}

	// Number <-> String
	if leftKind == ValueKindNumber && rightKind == ValueKindString {
		right = convertToNumber(right)
		rightKind = ValueKindNumber
		return left, right, leftKind, rightKind
	}
	if leftKind == ValueKindString && rightKind == ValueKindNumber {
		left = convertToNumber(left)
		leftKind = ValueKindNumber
		return left, right, leftKind, rightKind
	}

	// Boolean or Null -> Number
	if leftKind == ValueKindBoolean || leftKind == ValueKindNull {
		left = convertToNumber(left)
		return coerceTypes(left, right)
	}
	if rightKind == ValueKindBoolean || rightKind == ValueKindNull {
		right = convertToNumber(right)
		return coerceTypes(left, right)
	}

	// otherwise keep as is
	return left, right, leftKind, rightKind
}

// abstractEqual uses coerceTypes before comparing.
func abstractEqual(left, right interface{}) bool {
	left, right, leftKind, rightKind := coerceTypes(left, right)
	if leftKind != rightKind {
		return false
	}
	switch leftKind {
	case ValueKindNull:
		return true
	case ValueKindNumber:
		l := left.(float64)
		r := right.(float64)
		if isNaN(l) || isNaN(r) {
			return false
		}
		return l == r
	case ValueKindString:
		return strings.EqualFold(left.(string), right.(string))
	case ValueKindBoolean:
		return left.(bool) == right.(bool)
		// Compare object equality fails via panic
		// case ValueKindObject, ValueKindArray:
		// 	return left == right
	}
	return false
}

// abstractGreaterThan uses coerceTypes before comparing.
func abstractGreaterThan(left, right interface{}) bool {
	left, right, leftKind, rightKind := coerceTypes(left, right)
	if leftKind != rightKind {
		return false
	}
	switch leftKind {
	case ValueKindNumber:
		l := left.(float64)
		r := right.(float64)
		if isNaN(l) || isNaN(r) {
			return false
		}
		return l > r
	case ValueKindString:
		return strings.Compare(left.(string), right.(string)) > 0
	case ValueKindBoolean:
		return left.(bool) && !right.(bool)
	}
	return false
}

// abstractLessThan uses coerceTypes before comparing.
func abstractLessThan(left, right interface{}) bool {
	left, right, leftKind, rightKind := coerceTypes(left, right)
	if leftKind != rightKind {
		return false
	}
	switch leftKind {
	case ValueKindNumber:
		l := left.(float64)
		r := right.(float64)
		if isNaN(l) || isNaN(r) {
			return false
		}
		return l < r
	case ValueKindString:
		return strings.Compare(left.(string), right.(string)) < 0
	case ValueKindBoolean:
		return !left.(bool) && right.(bool)
	}
	return false
}

// convertToNumber converts a value to a float64 following JavaScript rules.
func convertToNumber(v interface{}) float64 {
	switch val := v.(type) {
	case nil:
		return 0
	case bool:
		if val {
			return 1
		}
		return 0
	case float64:
		return val
	case float32:
		return float64(val)
	case string:
		// parsenumber
		if val == "" {
			return float64(0)
		}
		if len(val) > 2 {
			switch val[:2] {
			case "0x", "0o":
				if i, err := strconv.ParseInt(val, 0, 32); err == nil {
					return float64(i)
				}
			}
		}
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return math.NaN()
	default:
		return math.NaN()
	}
}

// getKind returns the ValueKind for a Go value.
func getKind(v interface{}) ValueKind {
	switch v.(type) {
	case nil:
		return ValueKindNull
	case bool:
		return ValueKindBoolean
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return ValueKindNumber
	case string:
		return ValueKindString
	case []interface{}:
		return ValueKindArray
	case map[string]interface{}:
		return ValueKindObject
	default:
		return ValueKindObject
	}
}

// traceValue is a placeholder for tracing logic.
func (er *EvaluationResult) traceValue() {
	// No-op in this simplified implementation.
}

// --- End of file ---------------------------------------
