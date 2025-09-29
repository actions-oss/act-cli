package workflow

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// TokenKind represents the type of token returned by the lexer.
// The values mirror the C# TokenKind enum.
//
// Note: The names are kept identical to the C# implementation for
// easier mapping when porting the parser.
//
// The lexer is intentionally simple – it only tokenises the subset of
// expressions that are used in GitHub Actions workflow `if:` expressions.
// It does not evaluate the expression – that is left to the parser.

type TokenKind int

const (
	TokenKindStartGroup TokenKind = iota
	TokenKindStartIndex
	TokenKindEndGroup
	TokenKindEndIndex
	TokenKindSeparator
	TokenKindDereference
	TokenKindWildcard
	TokenKindLogicalOperator
	TokenKindNumber
	TokenKindString
	TokenKindBoolean
	TokenKindNull
	TokenKindPropertyName
	TokenKindFunction
	TokenKindNamedValue
	TokenKindStartParameters
	TokenKindEndParameters
	TokenKindUnexpected
)

// Token represents a single lexical token.
// Raw holds the original text, Value holds the parsed value when applicable.
// Index is the start position in the source string.
//
// The struct is intentionally minimal – it only contains what the parser
// needs. If you need more information (e.g. token length) you can add it.

type Token struct {
	Kind  TokenKind
	Raw   string
	Value interface{}
	Index int
}

// Lexer holds the state while tokenising an expression.
// It is a direct port of the C# LexicalAnalyzer.
//
// Flags can be used to enable/disable features – for now we only support
// a single flag that mirrors ExpressionFlags.DTExpressionsV1.
//
// The lexer is not thread‑safe – reuse a single instance per expression.

type Lexer struct {
	expr  string
	flags int
	index int
	last  *Token
	stack []TokenKind // unclosed start tokens
}

// NewLexer creates a new lexer for the given expression.
func NewLexer(expr string, flags int) *Lexer {
	return &Lexer{expr: expr, flags: flags}
}

func testTokenBoundary(c rune) bool {
	switch c {
	case '(', '[', ')', ']', ',', '.',
		'!', '>', '<', '=', '&', '|':
		return true
	default:
		return unicode.IsSpace(c)
	}
}

// Next returns the next token or nil if the end of the expression is reached.
func (l *Lexer) Next() *Token {
	// Skip whitespace
	for l.index < len(l.expr) && unicode.IsSpace(rune(l.expr[l.index])) {
		l.index++
	}
	if l.index >= len(l.expr) {
		return nil
	}

	c := l.expr[l.index]
	switch c {
	case '(':
		l.index++
		// Function call or logical grouping
		if l.last != nil && l.last.Kind == TokenKindFunction {
			return l.createToken(TokenKindStartParameters, "(")
		}
		if l.flags&FlagV1 != 0 {
			// V1 does not support grouping – treat as unexpected
			return l.createToken(TokenKindUnexpected, "(")
		}
		return l.createToken(TokenKindStartGroup, "(")
	case '[':
		l.index++
		return l.createToken(TokenKindStartIndex, "[")
	case ')':
		l.index++
		if len(l.stack) > 0 && l.stack[len(l.stack)-1] == TokenKindStartParameters {
			return l.createToken(TokenKindEndParameters, ")")
		}
		return l.createToken(TokenKindEndGroup, ")")
	case ']':
		l.index++
		return l.createToken(TokenKindEndIndex, "]")
	case ',':
		l.index++
		return l.createToken(TokenKindSeparator, ",")
	case '*':
		l.index++
		return l.createToken(TokenKindWildcard, "*")
	case '\'':
		return l.readString()
	case '!', '>', '<', '=', '&', '|':
		if l.flags&FlagV1 != 0 {
			l.index++
			return l.createToken(TokenKindUnexpected, string(c))
		}
		return l.readOperator()
	default:
		if c == '.' {
			// Could be number or dereference
			if l.last == nil || l.last.Kind == TokenKindSeparator || l.last.Kind == TokenKindStartGroup || l.last.Kind == TokenKindStartIndex || l.last.Kind == TokenKindStartParameters || l.last.Kind == TokenKindLogicalOperator {
				return l.readNumber()
			}
			l.index++
			return l.createToken(TokenKindDereference, ".")
		}
		if c == '-' || c == '+' || unicode.IsDigit(rune(c)) {
			return l.readNumber()
		}
		return l.readKeyword()
	}
}

// Helper to create a token and update lexer state.
func (l *Lexer) createToken(kind TokenKind, raw string) *Token {
	// Token order check
	if !l.checkLastToken(kind, raw) {
		// Illegal token sequence
		return &Token{Kind: TokenKindUnexpected, Raw: raw, Index: l.index}
	}
	tok := &Token{Kind: kind, Raw: raw, Index: l.index}
	//l.index++
	l.last = tok
	// Manage stack for grouping
	switch kind {
	case TokenKindStartGroup, TokenKindStartIndex, TokenKindStartParameters:
		l.stack = append(l.stack, kind)
	case TokenKindEndGroup, TokenKindEndIndex, TokenKindEndParameters:
		if len(l.stack) > 0 {
			l.stack = l.stack[:len(l.stack)-1]
		}
	}
	return tok
}

// checkLastToken verifies that the token sequence is legal based on the last token.
func (l *Lexer) checkLastToken(kind TokenKind, raw string) bool {
	// nil last token represented by nil
	var lastKind *TokenKind
	if l.last != nil {
		lastKind = &l.last.Kind
	}
	// Helper to check if lastKind is in allowed list
	allowed := func(allowedKinds ...TokenKind) bool {
		if lastKind == nil {
			for _, k := range allowedKinds {
				if k == TokenKindUnexpected { // placeholder for nil? but we use nil check
				}
			}
			return false
		}
		for _, k := range allowedKinds {
			if *lastKind == k {
				return true
			}
		}
		return false
	}
	// For nil last, we treat as no previous token
	// Define allowed previous kinds for each token kind
	switch kind {
	case TokenKindStartGroup:
		return lastKind == nil || allowed(TokenKindSeparator, TokenKindStartGroup, TokenKindStartParameters, TokenKindStartIndex, TokenKindLogicalOperator)
	case TokenKindStartIndex:
		return allowed(TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindStartParameters:
		return allowed(TokenKindFunction)
	case TokenKindEndGroup:
		return allowed(TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindNull, TokenKindBoolean, TokenKindNumber, TokenKindString, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindEndIndex:
		return allowed(TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindNull, TokenKindBoolean, TokenKindNumber, TokenKindString, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindEndParameters:
		return allowed(TokenKindStartParameters, TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindNull, TokenKindBoolean, TokenKindNumber, TokenKindString, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindSeparator:
		return allowed(TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindNull, TokenKindBoolean, TokenKindNumber, TokenKindString, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindWildcard:
		return allowed(TokenKindStartIndex, TokenKindDereference)
	case TokenKindDereference:
		return allowed(TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindLogicalOperator:
		if raw == "!" { // "!"
			return lastKind == nil || allowed(TokenKindSeparator, TokenKindStartGroup, TokenKindStartParameters, TokenKindStartIndex, TokenKindLogicalOperator)
		}
		return allowed(TokenKindEndGroup, TokenKindEndParameters, TokenKindEndIndex, TokenKindWildcard, TokenKindNull, TokenKindBoolean, TokenKindNumber, TokenKindString, TokenKindPropertyName, TokenKindNamedValue)
	case TokenKindNull, TokenKindBoolean, TokenKindNumber, TokenKindString:
		return lastKind == nil || allowed(TokenKindSeparator, TokenKindStartIndex, TokenKindStartGroup, TokenKindStartParameters, TokenKindLogicalOperator)
	case TokenKindPropertyName:
		return allowed(TokenKindDereference)
	case TokenKindFunction, TokenKindNamedValue:
		return lastKind == nil || allowed(TokenKindSeparator, TokenKindStartIndex, TokenKindStartGroup, TokenKindStartParameters, TokenKindLogicalOperator)
	default:
		return true
	}
}

// readNumber parses a numeric literal.
func (l *Lexer) readNumber() *Token {
	start := l.index
	periods := 0
	for l.index < len(l.expr) {
		ch := l.expr[l.index]
		if ch == '.' {
			periods++
		}
		if testTokenBoundary(rune(ch)) && ch != '.' {
			break
		}
		l.index++
	}
	raw := l.expr[start:l.index]
	if len(raw) > 2 {
		switch raw[:2] {
		case "0x", "0o":
			tok := l.createToken(TokenKindNumber, raw)
			if i, err := strconv.ParseInt(raw, 0, 32); err == nil {
				tok.Value = float64(i)
				return tok
			}
		}
	}
	// Try to parse as float64
	var val interface{} = raw
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		val = f
	}
	tok := l.createToken(TokenKindNumber, raw)
	tok.Value = val
	return tok
}

// readString parses a single‑quoted string literal.
func (l *Lexer) readString() *Token {
	start := l.index
	l.index++ // skip opening quote
	var sb strings.Builder
	closed := false
	for l.index < len(l.expr) {
		ch := l.expr[l.index]
		l.index++
		if ch == '\'' {
			if l.index < len(l.expr) && l.expr[l.index] == '\'' {
				// escaped quote
				sb.WriteByte('\'')
				l.index++
				continue
			}
			closed = true
			break
		}
		sb.WriteByte(ch)
	}
	raw := l.expr[start:l.index]
	tok := l.createToken(TokenKindString, raw)
	if closed {
		tok.Value = sb.String()
	} else {
		tok.Kind = TokenKindUnexpected
	}
	return tok
}

// readOperator parses logical operators (==, !=, >, >=, etc.).
func (l *Lexer) readOperator() *Token {
	start := l.index
	l.index++
	if l.index < len(l.expr) {
		two := l.expr[start : l.index+1]
		switch two {
		case "!=", ">=", "<=", "==", "&&", "||":
			l.index++
			return l.createToken(TokenKindLogicalOperator, two)
		}
	}
	ch := l.expr[start]
	switch ch {
	case '!', '>', '<':
		return l.createToken(TokenKindLogicalOperator, string(ch))
	}
	return l.createToken(TokenKindUnexpected, string(ch))
}

// readKeyword parses identifiers, booleans, null, etc.
func (l *Lexer) readKeyword() *Token {
	start := l.index
	for l.index < len(l.expr) && !unicode.IsSpace(rune(l.expr[l.index])) && !strings.ContainsRune("()[],.!<>==&|*", rune(l.expr[l.index])) {
		l.index++
	}
	raw := l.expr[start:l.index]
	if l.last != nil && l.last.Kind == TokenKindDereference {
		return l.createToken(TokenKindPropertyName, raw)
	}
	switch raw {
	case "true":
		tok := l.createToken(TokenKindBoolean, raw)
		tok.Value = true
		return tok
	case "false":
		tok := l.createToken(TokenKindBoolean, raw)
		tok.Value = false
		return tok
	case "null":
		return l.createToken(TokenKindNull, raw)
	case "NaN":
		tok := l.createToken(TokenKindNumber, raw)
		tok.Value = math.NaN()
		return tok
	case "Infinity":
		tok := l.createToken(TokenKindNumber, raw)
		tok.Value = math.Inf(1)
		return tok
	}
	if l.index < len(l.expr) && l.expr[l.index] == '(' {
		return l.createToken(TokenKindFunction, raw)
	}
	return l.createToken(TokenKindNamedValue, raw)
}

// Flag constants – only V1 is used for now.
const FlagV1 = 1

// UnclosedTokens returns the stack of unclosed start tokens.
func (l *Lexer) UnclosedTokens() []TokenKind {
	return l.stack
}
