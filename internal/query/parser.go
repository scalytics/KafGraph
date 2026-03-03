// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package query

import (
	"fmt"
	"strconv"
)

// Parser is a recursive descent parser for an OpenCypher subset.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a new Parser for the given tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse parses an input string into a Statement AST.
func Parse(input string) (*Statement, error) {
	tokens := Tokenize(input)
	p := NewParser(tokens)
	return p.ParseStatement()
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) expect(tt TokenType) (Token, error) {
	tok := p.advance()
	if tok.Type != tt {
		return tok, fmt.Errorf("expected %d, got %q at pos %d", tt, tok.Literal, tok.Pos)
	}
	return tok, nil
}

func (p *Parser) match(tt TokenType) bool {
	if p.peek().Type == tt {
		p.advance()
		return true
	}
	return false
}

// ParseStatement parses a full Cypher statement.
func (p *Parser) ParseStatement() (*Statement, error) {
	stmt := &Statement{}
	for p.peek().Type != TokenEOF {
		clause, err := p.parseClause()
		if err != nil {
			return nil, err
		}
		stmt.Clauses = append(stmt.Clauses, clause)
	}
	if len(stmt.Clauses) == 0 {
		return nil, fmt.Errorf("empty statement")
	}
	return stmt, nil
}

func (p *Parser) parseClause() (Clause, error) {
	switch p.peek().Type {
	case TokenMATCH:
		return p.parseMatch()
	case TokenWHERE:
		return p.parseWhere()
	case TokenRETURN:
		return p.parseReturn()
	case TokenCREATE:
		return p.parseCreate()
	case TokenMERGE:
		return p.parseMerge()
	case TokenSET:
		return p.parseSet()
	case TokenDELETE:
		return p.parseDelete()
	case TokenCALL:
		return p.parseCall()
	default:
		return nil, fmt.Errorf("unexpected token %q at pos %d", p.peek().Literal, p.peek().Pos)
	}
}

func (p *Parser) parseMatch() (*MatchClause, error) {
	p.advance() // consume MATCH
	clause := &MatchClause{}
	pattern, err := p.parsePattern()
	if err != nil {
		return nil, err
	}
	clause.Patterns = append(clause.Patterns, pattern)
	for p.match(TokenComma) {
		pattern, err = p.parsePattern()
		if err != nil {
			return nil, err
		}
		clause.Patterns = append(clause.Patterns, pattern)
	}
	return clause, nil
}

func (p *Parser) parsePattern() (Pattern, error) {
	pat := Pattern{}

	node, err := p.parseNodePattern()
	if err != nil {
		return pat, err
	}
	pat.Nodes = append(pat.Nodes, node)

	// Check for relationship chain
	for p.peek().Type == TokenDash || p.peek().Type == TokenLArrow {
		edge, nextNode, err := p.parseRelAndNode()
		if err != nil {
			return pat, err
		}
		pat.Edges = append(pat.Edges, edge)
		pat.Nodes = append(pat.Nodes, nextNode)
	}
	return pat, nil
}

func (p *Parser) parseNodePattern() (NodePattern, error) {
	np := NodePattern{}
	if _, err := p.expect(TokenLParen); err != nil {
		return np, err
	}

	// Optional variable
	if p.peek().Type == TokenIdent {
		np.Variable = p.advance().Literal
	}

	// Optional label
	if p.match(TokenColon) {
		tok, err := p.expect(TokenIdent)
		if err != nil {
			return np, fmt.Errorf("expected label: %w", err)
		}
		np.Label = tok.Literal
	}

	// Optional properties
	if p.peek().Type == TokenLBrace {
		props, err := p.parseMapLiteral()
		if err != nil {
			return np, err
		}
		np.Properties = props
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return np, err
	}
	return np, nil
}

func (p *Parser) parseRelAndNode() (EdgePattern, NodePattern, error) {
	ep := EdgePattern{}

	if p.peek().Type == TokenLArrow {
		// <-[:LABEL]-(node)
		ep.Direction = EdgeLeft
		p.advance() // consume <-
		if p.peek().Type == TokenLBracket {
			if err := p.parseRelDetail(&ep); err != nil {
				return ep, NodePattern{}, err
			}
		}
		if _, err := p.expect(TokenDash); err != nil {
			return ep, NodePattern{}, err
		}
	} else {
		// -[:LABEL]->(node)
		ep.Direction = EdgeRight
		p.advance() // consume -
		if p.peek().Type == TokenLBracket {
			if err := p.parseRelDetail(&ep); err != nil {
				return ep, NodePattern{}, err
			}
		}
		if _, err := p.expect(TokenArrow); err != nil {
			return ep, NodePattern{}, err
		}
	}

	node, err := p.parseNodePattern()
	return ep, node, err
}

func (p *Parser) parseRelDetail(ep *EdgePattern) error {
	p.advance() // consume [
	if p.peek().Type == TokenIdent {
		ep.Variable = p.advance().Literal
	}
	if p.match(TokenColon) {
		tok, err := p.expect(TokenIdent)
		if err != nil {
			return fmt.Errorf("expected relationship label: %w", err)
		}
		ep.Label = tok.Literal
	}
	_, err := p.expect(TokenRBracket)
	return err
}

func (p *Parser) parseMapLiteral() (map[string]Expr, error) {
	p.advance() // consume {
	props := make(map[string]Expr)
	if p.peek().Type == TokenRBrace {
		p.advance()
		return props, nil
	}
	for {
		key, err := p.expect(TokenIdent)
		if err != nil {
			return nil, fmt.Errorf("expected property key: %w", err)
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		props[key.Literal] = val
		if !p.match(TokenComma) {
			break
		}
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return props, nil
}

func (p *Parser) parseWhere() (*WhereClause, error) {
	p.advance() // consume WHERE
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &WhereClause{Expr: expr}, nil
}

func (p *Parser) parseReturn() (*ReturnClause, error) {
	p.advance() // consume RETURN
	clause := &ReturnClause{}

	// Parse return items
	item, err := p.parseReturnItem()
	if err != nil {
		return nil, err
	}
	clause.Items = append(clause.Items, item)
	for p.match(TokenComma) {
		item, err = p.parseReturnItem()
		if err != nil {
			return nil, err
		}
		clause.Items = append(clause.Items, item)
	}

	// Optional ORDER BY
	if p.peek().Type == TokenORDER {
		p.advance() // ORDER
		if _, err := p.expect(TokenBY); err != nil {
			return nil, err
		}
		for {
			orderExpr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			desc := false
			if p.match(TokenDESC) {
				desc = true
			} else {
				p.match(TokenASC) // optional ASC
			}
			clause.OrderBy = append(clause.OrderBy, OrderItem{Expr: orderExpr, Desc: desc})
			if !p.match(TokenComma) {
				break
			}
		}
	}

	// Optional LIMIT
	if p.match(TokenLIMIT) {
		tok, err := p.expect(TokenInt)
		if err != nil {
			return nil, err
		}
		v, _ := strconv.Atoi(tok.Literal)
		clause.Limit = &v
	}

	// Optional SKIP
	if p.match(TokenSKIP) {
		tok, err := p.expect(TokenInt)
		if err != nil {
			return nil, err
		}
		v, _ := strconv.Atoi(tok.Literal)
		clause.Skip = &v
	}

	return clause, nil
}

func (p *Parser) parseReturnItem() (ReturnItem, error) {
	expr, err := p.parseExpr()
	if err != nil {
		return ReturnItem{}, err
	}
	item := ReturnItem{Expr: expr}
	if p.match(TokenAS) {
		tok, err := p.expect(TokenIdent)
		if err != nil {
			return item, err
		}
		item.Alias = tok.Literal
	}
	return item, nil
}

func (p *Parser) parseCreate() (*CreateClause, error) {
	p.advance() // consume CREATE
	clause := &CreateClause{}
	pattern, err := p.parsePattern()
	if err != nil {
		return nil, err
	}
	clause.Patterns = append(clause.Patterns, pattern)
	for p.match(TokenComma) {
		pattern, err = p.parsePattern()
		if err != nil {
			return nil, err
		}
		clause.Patterns = append(clause.Patterns, pattern)
	}
	return clause, nil
}

func (p *Parser) parseMerge() (*MergeClause, error) {
	p.advance() // consume MERGE
	pattern, err := p.parsePattern()
	if err != nil {
		return nil, err
	}
	return &MergeClause{Pattern: pattern}, nil
}

func (p *Parser) parseSet() (*SetClause, error) {
	p.advance() // consume SET
	clause := &SetClause{}
	for {
		item, err := p.parseSetItem()
		if err != nil {
			return nil, err
		}
		clause.Items = append(clause.Items, item)
		if !p.match(TokenComma) {
			break
		}
	}
	return clause, nil
}

func (p *Parser) parseSetItem() (SetItem, error) {
	varTok, err := p.expect(TokenIdent)
	if err != nil {
		return SetItem{}, err
	}
	if _, err := p.expect(TokenDot); err != nil {
		return SetItem{}, err
	}
	propTok, err := p.expect(TokenIdent)
	if err != nil {
		return SetItem{}, err
	}
	if _, err := p.expect(TokenEQ); err != nil {
		return SetItem{}, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return SetItem{}, err
	}
	return SetItem{Variable: varTok.Literal, Property: propTok.Literal, Value: val}, nil
}

func (p *Parser) parseDelete() (*DeleteClause, error) {
	p.advance() // consume DELETE
	clause := &DeleteClause{}
	tok, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	clause.Variables = append(clause.Variables, tok.Literal)
	for p.match(TokenComma) {
		tok, err = p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		clause.Variables = append(clause.Variables, tok.Literal)
	}
	return clause, nil
}

func (p *Parser) parseCall() (*CallClause, error) {
	p.advance() // consume CALL
	clause := &CallClause{}

	// Parse procedure name: ident.ident.ident
	nameTok, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	name := nameTok.Literal
	for p.match(TokenDot) {
		tok, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		name += "." + tok.Literal
	}
	clause.Procedure = name

	// Parse arguments
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	if p.peek().Type != TokenRParen {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		clause.Args = append(clause.Args, arg)
		for p.match(TokenComma) {
			arg, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
			clause.Args = append(clause.Args, arg)
		}
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	// Optional YIELD
	if p.match(TokenYIELD) {
		tok, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		clause.YieldVars = append(clause.YieldVars, tok.Literal)
		for p.match(TokenComma) {
			tok, err = p.expect(TokenIdent)
			if err != nil {
				return nil, err
			}
			clause.YieldVars = append(clause.YieldVars, tok.Literal)
		}
	}

	return clause, nil
}

// Expression parsing with precedence: OR → AND → NOT → comparison → atom

func (p *Parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenOR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "OR", Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenAND {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "AND", Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (Expr, error) {
	if p.peek().Type == TokenNOT {
		p.advance()
		expr, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Expr: expr}, nil
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() (Expr, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	switch p.peek().Type {
	case TokenEQ, TokenNEQ, TokenLT, TokenGT, TokenLTE, TokenGTE:
		op := p.advance().Literal
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: op, Right: right}, nil
	case TokenCONTAINS:
		p.advance()
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: "CONTAINS", Right: right}, nil
	case TokenIN:
		p.advance()
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: "IN", Right: right}, nil
	}
	return left, nil
}

func (p *Parser) parseAtom() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenStar:
		p.advance()
		return &StarExpr{}, nil

	case TokenInt:
		p.advance()
		v, _ := strconv.ParseInt(tok.Literal, 10, 64)
		return &LiteralExpr{Value: v}, nil

	case TokenFloat:
		p.advance()
		v, _ := strconv.ParseFloat(tok.Literal, 64)
		return &LiteralExpr{Value: v}, nil

	case TokenString:
		p.advance()
		return &LiteralExpr{Value: tok.Literal}, nil

	case TokenTRUE:
		p.advance()
		return &LiteralExpr{Value: true}, nil

	case TokenFALSE:
		p.advance()
		return &LiteralExpr{Value: false}, nil

	case TokenNULL:
		p.advance()
		return &LiteralExpr{Value: nil}, nil

	case TokenParam:
		p.advance()
		return &ParamExpr{Name: tok.Literal[1:]}, nil // strip $

	case TokenLBracket:
		return p.parseListLiteral()

	case TokenLParen:
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil

	case TokenIdent, TokenCOUNT:
		name := p.advance().Literal
		// Function call
		if p.peek().Type == TokenLParen {
			p.advance() // (
			var args []Expr
			if p.peek().Type != TokenRParen {
				if p.peek().Type == TokenStar {
					p.advance()
					args = append(args, &StarExpr{})
				} else {
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
				}
				for p.match(TokenComma) {
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
				}
			}
			if _, err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
			return &FuncCallExpr{Name: name, Args: args}, nil
		}
		// Property access
		if p.peek().Type == TokenDot {
			p.advance()
			prop, err := p.expect(TokenIdent)
			if err != nil {
				return nil, err
			}
			return &PropertyExpr{Variable: name, Property: prop.Literal}, nil
		}
		return &IdentExpr{Name: name}, nil

	case TokenDash:
		// Negative number: -42
		p.advance()
		if p.peek().Type == TokenInt {
			tok := p.advance()
			v, _ := strconv.ParseInt(tok.Literal, 10, 64)
			return &LiteralExpr{Value: -v}, nil
		}
		if p.peek().Type == TokenFloat {
			tok := p.advance()
			v, _ := strconv.ParseFloat(tok.Literal, 64)
			return &LiteralExpr{Value: -v}, nil
		}
		return nil, fmt.Errorf("unexpected token after - at pos %d", tok.Pos)
	}

	return nil, fmt.Errorf("unexpected token %q at pos %d", tok.Literal, tok.Pos)
}

func (p *Parser) parseListLiteral() (Expr, error) {
	p.advance() // consume [
	var elements []Expr
	if p.peek().Type != TokenRBracket {
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
		for p.match(TokenComma) {
			elem, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)
		}
	}
	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return &ListExpr{Elements: elements}, nil
}
