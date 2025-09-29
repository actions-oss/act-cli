package workflow

import (
	"errors"
	"fmt"
	"strings"
)

// Node represents a node in the expression tree.
// It is intentionally minimal – only the fields needed for the parser.
// Users can extend it with more information if required.

type Node interface {
	String() string
}

// ValueNode represents a literal value (number, string, boolean, null) or a named value.
// The Kind field indicates the type.
// For named values the Value is nil.

type ValueNode struct {
	Kind  TokenKind
	Value interface{}
}

// FunctionNode represents a function call with arguments.

type FunctionNode struct {
	Name string
	Args []Node
}

// BinaryNode represents a binary operator.

type BinaryNode struct {
	Op    string
	Left  Node
	Right Node
}

// UnaryNode represents a unary operator.

type UnaryNode struct {
	Op      string
	Operand Node
}

// Parser holds the lexer and the stacks used by the shunting‑yard algorithm.

type Parser struct {
	lexer  *Lexer
	tokens []Token
	pos    int
	ops    []OpToken
	vals   []Node
}

type OpToken struct {
	Token
	StartPos int
}

// precedence maps operator raw values to precedence levels.
var precedence = map[string]int{
	"||": 1,
	"&&": 2,
	"==": 3,
	"!=": 3,
	">":  4,
	"<":  4,
	">=": 4,
	"<=": 4,
	"!":  5,
	".":  6, // dereference operator
	"[":  6, // index operator
}

// Parse parses the expression and returns the root node.
func Parse(expression string) (Node, error) {
	lexer := NewLexer(expression, 0)
	p := &Parser{lexer: lexer}
	// Tokenise all tokens
	for {
		tok := lexer.Next()
		if tok == nil {
			break
		}
		if tok.Kind == TokenKindUnexpected {
			return nil, fmt.Errorf("unexpected token %s at position %d", tok.Raw, tok.Index)
		}
		p.tokens = append(p.tokens, *tok)
	}
	// Shunting‑yard algorithm
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		p.pos++
		switch tok.Kind {
		case TokenKindNumber, TokenKindString, TokenKindBoolean, TokenKindNull:
			p.pushValue(&ValueNode{Kind: tok.Kind, Value: tok.Value})
		case TokenKindNamedValue, TokenKindPropertyName:
			p.pushValue(&ValueNode{Kind: tok.Kind, Value: tok.Raw})
		// In the shunting‑yard loop, treat TokenKindDereference as a unary operator
		case TokenKindDereference:
			// push as an operator with high precedence
			for len(p.ops) > 0 {
				top := p.ops[len(p.ops)-1]
				if precedence[top.Raw] >= precedence[tok.Raw] {
					if err := p.popOp(); err != nil {
						return nil, err
					}
				} else {
					break
				}
			}
			p.pushOp(tok)
		case TokenKindWildcard:
			p.pushValue(&ValueNode{Kind: tok.Kind, Value: tok.Raw})
		case TokenKindFunction:
			p.pushFunc(tok, len(p.vals))
		case TokenKindStartParameters:
			p.pushOp(tok)
		case TokenKindSeparator:
			for len(p.ops) > 0 && p.ops[len(p.ops)-1].Kind != TokenKindStartParameters {
				if err := p.popOp(); err != nil {
					return nil, err
				}
			}
		case TokenKindEndParameters:
			for len(p.ops) > 0 && p.ops[len(p.ops)-1].Kind != TokenKindStartParameters {
				if err := p.popOp(); err != nil {
					return nil, err
				}
			}
			if len(p.ops) == 0 {
				return nil, errors.New("mismatched parentheses")
			}
			// pop the start parameters
			p.ops = p.ops[:len(p.ops)-1]
			// create function node
			fnTok := p.ops[len(p.ops)-1]
			if fnTok.Kind != TokenKindFunction {
				return nil, errors.New("expected function token")
			}
			p.ops = p.ops[:len(p.ops)-1]
			// collect arguments
			args := []Node{}
			for len(p.vals) > fnTok.StartPos {
				args = append([]Node{p.vals[len(p.vals)-1]}, args...)
				p.vals = p.vals[:len(p.vals)-1]
			}
			p.pushValue(&FunctionNode{Name: fnTok.Raw, Args: args})
		case TokenKindStartGroup:
			p.pushOp(tok)
		case TokenKindEndGroup:
			for len(p.ops) > 0 && p.ops[len(p.ops)-1].Kind != TokenKindStartGroup {
				if err := p.popOp(); err != nil {
					return nil, err
				}
			}
			if len(p.ops) == 0 {
				return nil, errors.New("mismatched parentheses")
			}
			p.ops = p.ops[:len(p.ops)-1]
		case TokenKindLogicalOperator:
			for len(p.ops) > 0 {
				top := p.ops[len(p.ops)-1]
				if precedence[top.Raw] >= precedence[tok.Raw] {
					if err := p.popOp(); err != nil {
						return nil, err
					}
				} else {
					break
				}
			}
			p.pushOp(tok)
		case TokenKindStartIndex:
			for len(p.ops) > 0 {
				top := p.ops[len(p.ops)-1]
				if precedence[top.Raw] >= precedence[tok.Raw] {
					if err := p.popOp(); err != nil {
						return nil, err
					}
				} else {
					break
				}
			}
			p.pushOp(tok)
		case TokenKindEndIndex:
			for len(p.ops) > 0 && p.ops[len(p.ops)-1].Kind != TokenKindStartIndex {
				if err := p.popOp(); err != nil {
					return nil, err
				}
			}
			if len(p.ops) == 0 {
				return nil, errors.New("mismatched parentheses")
			}
			// pop the start parameters
			p.ops = p.ops[:len(p.ops)-1]
			right := p.vals[len(p.vals)-1]
			p.vals = p.vals[:len(p.vals)-1]
			left := p.vals[len(p.vals)-1]
			p.vals = p.vals[:len(p.vals)-1]
			p.vals = append(p.vals, &BinaryNode{Op: "[", Left: left, Right: right})
		}
	}
	for len(p.ops) > 0 {
		if err := p.popOp(); err != nil {
			return nil, err
		}
	}
	if len(p.vals) != 1 {
		return nil, errors.New("invalid expression")
	}
	return p.vals[0], nil
}

func (p *Parser) pushValue(v Node) {
	p.vals = append(p.vals, v)
}

func (p *Parser) pushOp(t Token) {
	p.ops = append(p.ops, OpToken{Token: t})
}

func (p *Parser) pushFunc(t Token, start int) {
	p.ops = append(p.ops, OpToken{Token: t, StartPos: start})
}

func (p *Parser) popOp() error {
	if len(p.ops) == 0 {
		return nil
	}
	op := p.ops[len(p.ops)-1]
	p.ops = p.ops[:len(p.ops)-1]
	switch op.Kind {
	case TokenKindLogicalOperator:
		if op.Raw == "!" {
			if len(p.vals) < 1 {
				return errors.New("insufficient operands")
			}
			right := p.vals[len(p.vals)-1]
			p.vals = p.vals[:len(p.vals)-1]
			p.vals = append(p.vals, &UnaryNode{Op: op.Raw, Operand: right})
		} else {
			if len(p.vals) < 2 {
				return errors.New("insufficient operands")
			}
			right := p.vals[len(p.vals)-1]
			left := p.vals[len(p.vals)-2]
			p.vals = p.vals[:len(p.vals)-2]
			p.vals = append(p.vals, &BinaryNode{Op: op.Raw, Left: left, Right: right})
		}
	case TokenKindStartParameters:
		// unary operator '!' handled elsewhere
	case TokenKindDereference:
		if len(p.vals) < 2 {
			return errors.New("insufficient operands")
		}
		right := p.vals[len(p.vals)-1]
		left := p.vals[len(p.vals)-2]
		p.vals = p.vals[:len(p.vals)-2]
		p.vals = append(p.vals, &BinaryNode{Op: ".", Left: left, Right: right})
	}
	return nil
}

// String returns a string representation of the node.
func (n *ValueNode) String() string { return fmt.Sprintf("%v", n.Value) }

// String returns a string representation of the node.
func (n *FunctionNode) String() string {
	return fmt.Sprintf("%s(%s)", n.Name, strings.Join(funcArgs(n.Args), ", "))
}

func funcArgs(args []Node) []string {
	res := []string{}
	for _, a := range args {
		res = append(res, a.String())
	}
	return res
}

// String returns a string representation of the node.
func (n *BinaryNode) String() string {
	return fmt.Sprintf("(%s %s %s)", n.Left.String(), n.Op, n.Right.String())
}

// String returns a string representation of the node.
func (n *UnaryNode) String() string { return fmt.Sprintf("(%s%s)", n.Op, n.Operand.String()) }
