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
	"strings"
	"unicode"
)

// Lexer tokenizes a Cypher query string.
type Lexer struct {
	input []rune
	pos   int
}

// NewLexer creates a new Lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: []rune(input)}
}

// Tokenize returns all tokens from the input.
func Tokenize(input string) []Token {
	l := NewLexer(input)
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF || tok.Type == TokenIllegal {
			break
		}
	}
	return tokens
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) advance() rune {
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

// Next returns the next token.
func (l *Lexer) Next() Token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Pos: l.pos}
	}

	start := l.pos
	ch := l.peek()

	// String literals
	if ch == '\'' || ch == '"' {
		return l.readString(ch)
	}

	// Numbers
	if unicode.IsDigit(ch) {
		return l.readNumber()
	}

	// Identifiers and keywords
	if unicode.IsLetter(ch) || ch == '_' {
		return l.readIdentOrKeyword()
	}

	// Parameter
	if ch == '$' {
		l.advance()
		tok := l.readIdentOrKeyword()
		tok.Type = TokenParam
		tok.Literal = "$" + tok.Literal
		tok.Pos = start
		return tok
	}

	// Symbols
	l.advance()
	switch ch {
	case '(':
		return Token{Type: TokenLParen, Literal: "(", Pos: start}
	case ')':
		return Token{Type: TokenRParen, Literal: ")", Pos: start}
	case '[':
		return Token{Type: TokenLBracket, Literal: "[", Pos: start}
	case ']':
		return Token{Type: TokenRBracket, Literal: "]", Pos: start}
	case '{':
		return Token{Type: TokenLBrace, Literal: "{", Pos: start}
	case '}':
		return Token{Type: TokenRBrace, Literal: "}", Pos: start}
	case ':':
		return Token{Type: TokenColon, Literal: ":", Pos: start}
	case ',':
		return Token{Type: TokenComma, Literal: ",", Pos: start}
	case '.':
		return Token{Type: TokenDot, Literal: ".", Pos: start}
	case '*':
		return Token{Type: TokenStar, Literal: "*", Pos: start}
	case '|':
		return Token{Type: TokenPipe, Literal: "|", Pos: start}
	case '=':
		return Token{Type: TokenEQ, Literal: "=", Pos: start}
	case '<':
		if l.peek() == '>' {
			l.advance()
			return Token{Type: TokenNEQ, Literal: "<>", Pos: start}
		}
		if l.peek() == '=' {
			l.advance()
			return Token{Type: TokenLTE, Literal: "<=", Pos: start}
		}
		if l.peek() == '-' {
			l.advance()
			return Token{Type: TokenLArrow, Literal: "<-", Pos: start}
		}
		return Token{Type: TokenLT, Literal: "<", Pos: start}
	case '>':
		if l.peek() == '=' {
			l.advance()
			return Token{Type: TokenGTE, Literal: ">=", Pos: start}
		}
		return Token{Type: TokenGT, Literal: ">", Pos: start}
	case '-':
		if l.peek() == '>' {
			l.advance()
			return Token{Type: TokenArrow, Literal: "->", Pos: start}
		}
		return Token{Type: TokenDash, Literal: "-", Pos: start}
	}

	return Token{Type: TokenIllegal, Literal: string(ch), Pos: start}
}

func (l *Lexer) readString(quote rune) Token {
	start := l.pos
	l.advance() // skip opening quote
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if ch == quote {
			return Token{Type: TokenString, Literal: sb.String(), Pos: start}
		}
		if ch == '\\' && l.pos < len(l.input) {
			next := l.advance()
			switch next {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteRune(next)
			}
			continue
		}
		sb.WriteRune(ch)
	}
	return Token{Type: TokenIllegal, Literal: sb.String(), Pos: start}
}

func (l *Lexer) readNumber() Token {
	start := l.pos
	isFloat := false
	for l.pos < len(l.input) && (unicode.IsDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		if l.input[l.pos] == '.' {
			if isFloat {
				break
			}
			isFloat = true
		}
		l.pos++
	}
	lit := string(l.input[start:l.pos])
	if isFloat {
		return Token{Type: TokenFloat, Literal: lit, Pos: start}
	}
	return Token{Type: TokenInt, Literal: lit, Pos: start}
}

func (l *Lexer) readIdentOrKeyword() Token {
	start := l.pos
	for l.pos < len(l.input) && (unicode.IsLetter(l.input[l.pos]) || unicode.IsDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	lit := string(l.input[start:l.pos])
	upper := strings.ToUpper(lit)
	tokType := LookupIdent(upper)
	return Token{Type: tokType, Literal: lit, Pos: start}
}
