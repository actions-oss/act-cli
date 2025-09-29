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

func precedence(tkn Token) int {
	switch tkn.Kind {
	case TokenKindStartGroup:
		return 20
	case TokenKindStartIndex, TokenKindStartParameters, TokenKindDereference:
		return 19
	case TokenKindLogicalOperator:
		switch tkn.Raw {
		case "!":
			return 16
		case ">", ">=", "<", "<=":
			return 11
		case "==", "!=":
			return 10
		case "&&":
			return 6
		case "||":
			return 5
		}
	case TokenKindEndGroup, TokenKindEndIndex, TokenKindEndParameters, TokenKindSeparator:
		return 1
	}
	return 0
}

// Parse parses the expression and returns the root node.
func Parse(expression string) (Node, error) {
	lexer := NewLexer(expression, 0)
	p := &Parser{}
	// Tokenise all tokens
	if err := p.initWithLexer(lexer); err != nil {
		return nil, err
	}
	return p.parse()
}

func (p *Parser) parse() (Node, error) {
	// Shunting‑yard algorithm
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		p.pos++
		switch tok.Kind {
		case TokenKindNumber, TokenKindString, TokenKindBoolean, TokenKindNull:
			p.pushValue(&ValueNode{Kind: tok.Kind, Value: tok.Value})
		case TokenKindNamedValue, TokenKindPropertyName, TokenKindWildcard:
			p.pushValue(&ValueNode{Kind: tok.Kind, Value: tok.Raw})
		// In the shunting‑yard loop, treat TokenKindDereference as a unary operator
		case TokenKindLogicalOperator, TokenKindDereference:
			if err := p.pushBinaryOperator(tok); err != nil {
				return nil, err
			}
		case TokenKindFunction:
			p.pushFunc(tok, len(p.vals))
		case TokenKindStartParameters, TokenKindStartGroup, TokenKindStartIndex:
			p.pushOp(tok)
		case TokenKindSeparator:
			if err := p.popGroup(TokenKindStartParameters); err != nil {
				return nil, err
			}
		case TokenKindEndParameters:
			if err := p.pushFuncValue(); err != nil {
				return nil, err
			}
		case TokenKindEndGroup:
			if err := p.popGroup(TokenKindStartGroup); err != nil {
				return nil, err
			}

			p.ops = p.ops[:len(p.ops)-1]
		case TokenKindEndIndex:
			if err := p.popGroup(TokenKindStartIndex); err != nil {
				return nil, err
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

func (p *Parser) pushFuncValue() error {
	if err := p.popGroup(TokenKindStartParameters); err != nil {
		return err
	}

	// pop the start parameters
	p.ops = p.ops[:len(p.ops)-1]
	// create function node
	fnTok := p.ops[len(p.ops)-1]
	if fnTok.Kind != TokenKindFunction {
		return errors.New("expected function token")
	}
	p.ops = p.ops[:len(p.ops)-1]
	// collect arguments
	args := []Node{}
	for len(p.vals) > fnTok.StartPos {
		args = append([]Node{p.vals[len(p.vals)-1]}, args...)
		p.vals = p.vals[:len(p.vals)-1]
	}
	p.pushValue(&FunctionNode{Name: fnTok.Raw, Args: args})
	return nil
}

func (p *Parser) pushBinaryOperator(tok Token) error {
	// push as an operator
	// for len(p.ops) > 0 {
	// 	top := p.ops[len(p.ops)-1]
	// 	if precedence(top.Token) >= precedence(tok) &&
	// 		top.Kind != TokenKindStartGroup &&
	// 		top.Kind != TokenKindStartIndex &&
	// 		top.Kind != TokenKindStartParameters &&
	// 		top.Kind != TokenKindSeparator {
	// 		if err := p.popOp(); err != nil {
	// 			return err
	// 		}
	// 	} else {
	// 		break
	// 	}
	// }
	p.pushOp(tok)
	return nil
}

func (p *Parser) initWithLexer(lexer *Lexer) error {
	p.lexer = lexer
	for {
		tok := lexer.Next()
		if tok == nil {
			break
		}
		if tok.Kind == TokenKindUnexpected {
			return fmt.Errorf("unexpected token %s at position %d", tok.Raw, tok.Index)
		}
		p.tokens = append(p.tokens, *tok)
	}
	return nil
}

func (p *Parser) popGroup(kind TokenKind) error {
	for len(p.ops) > 0 && p.ops[len(p.ops)-1].Kind != kind {
		if err := p.popOp(); err != nil {
			return err
		}
	}
	if len(p.ops) == 0 {
		return errors.New("mismatched parentheses")
	}
	return nil
}

func (p *Parser) pushValue(v Node) {
	p.vals = append(p.vals, v)
}

func (p *Parser) pushOp(t Token) {
	for len(p.ops) > 0 {
		top := p.ops[len(p.ops)-1]
		if precedence(top.Token) >= precedence(t) &&
			top.Kind != TokenKindStartGroup &&
			top.Kind != TokenKindStartIndex &&
			top.Kind != TokenKindStartParameters &&
			top.Kind != TokenKindSeparator {
			if err := p.popOp(); err != nil {
				panic(err)
			}
		} else {
			break
		}
	}
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

func VisitNode(exprNode Node, callback func(node Node)) {
	callback(exprNode)
	switch node := exprNode.(type) {
	case *FunctionNode:
		for _, arg := range node.Args {
			VisitNode(arg, callback)
		}
	case *UnaryNode:
		VisitNode(node.Operand, callback)
	case *BinaryNode:
		VisitNode(node.Left, callback)
		VisitNode(node.Right, callback)
	}
}
