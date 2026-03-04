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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexerKeywords(t *testing.T) {
	tokens := Tokenize("MATCH WHERE RETURN CREATE MERGE SET DELETE ORDER BY LIMIT SKIP AS")
	expected := []TokenType{
		TokenMATCH, TokenWHERE, TokenRETURN, TokenCREATE, TokenMERGE,
		TokenSET, TokenDELETE, TokenORDER, TokenBY, TokenLIMIT, TokenSKIP, TokenAS, TokenEOF,
	}
	require.Len(t, tokens, len(expected))
	for i, tok := range tokens {
		assert.Equal(t, expected[i], tok.Type, "token %d", i)
	}
}

func TestLexerCaseInsensitive(t *testing.T) {
	tokens := Tokenize("match Match MATCH")
	assert.Equal(t, TokenMATCH, tokens[0].Type)
	assert.Equal(t, TokenMATCH, tokens[1].Type)
	assert.Equal(t, TokenMATCH, tokens[2].Type)
}

func TestLexerIdentifiers(t *testing.T) {
	tokens := Tokenize("n myVar _under foo123")
	for i := range 4 {
		assert.Equal(t, TokenIdent, tokens[i].Type)
	}
	assert.Equal(t, "n", tokens[0].Literal)
	assert.Equal(t, "myVar", tokens[1].Literal)
	assert.Equal(t, "_under", tokens[2].Literal)
	assert.Equal(t, "foo123", tokens[3].Literal)
}

func TestLexerStringSingleQuote(t *testing.T) {
	tokens := Tokenize("'hello world'")
	require.Len(t, tokens, 2) // string + EOF
	assert.Equal(t, TokenString, tokens[0].Type)
	assert.Equal(t, "hello world", tokens[0].Literal)
}

func TestLexerStringDoubleQuote(t *testing.T) {
	tokens := Tokenize(`"hello world"`)
	assert.Equal(t, TokenString, tokens[0].Type)
	assert.Equal(t, "hello world", tokens[0].Literal)
}

func TestLexerStringEscape(t *testing.T) {
	tokens := Tokenize(`'hello\nworld'`)
	assert.Equal(t, TokenString, tokens[0].Type)
	assert.Equal(t, "hello\nworld", tokens[0].Literal)
}

func TestLexerIntegers(t *testing.T) {
	tokens := Tokenize("42 0 100")
	assert.Equal(t, TokenInt, tokens[0].Type)
	assert.Equal(t, "42", tokens[0].Literal)
	assert.Equal(t, TokenInt, tokens[1].Type)
	assert.Equal(t, TokenInt, tokens[2].Type)
}

func TestLexerFloats(t *testing.T) {
	tokens := Tokenize("3.14 0.5")
	assert.Equal(t, TokenFloat, tokens[0].Type)
	assert.Equal(t, "3.14", tokens[0].Literal)
	assert.Equal(t, TokenFloat, tokens[1].Type)
}

func TestLexerSymbols(t *testing.T) {
	tokens := Tokenize("( ) [ ] { } : , . * = <> <= >=")
	expected := []TokenType{
		TokenLParen, TokenRParen, TokenLBracket, TokenRBracket,
		TokenLBrace, TokenRBrace, TokenColon, TokenComma, TokenDot, TokenStar,
		TokenEQ, TokenNEQ, TokenLTE, TokenGTE, TokenEOF,
	}
	require.Len(t, tokens, len(expected))
	for i, tok := range tokens {
		assert.Equal(t, expected[i], tok.Type, "token %d: %s", i, tok.Literal)
	}
}

func TestLexerArrows(t *testing.T) {
	tokens := Tokenize("-> <- -")
	assert.Equal(t, TokenArrow, tokens[0].Type)
	assert.Equal(t, TokenLArrow, tokens[1].Type)
	assert.Equal(t, TokenDash, tokens[2].Type)
}

func TestLexerParam(t *testing.T) {
	tokens := Tokenize("$name $value")
	assert.Equal(t, TokenParam, tokens[0].Type)
	assert.Equal(t, "$name", tokens[0].Literal)
	assert.Equal(t, TokenParam, tokens[1].Type)
	assert.Equal(t, "$value", tokens[1].Literal)
}

func TestLexerMatchPattern(t *testing.T) {
	tokens := Tokenize("MATCH (n:Agent {name: 'alice'})")
	expected := []TokenType{
		TokenMATCH, TokenLParen, TokenIdent, TokenColon, TokenIdent,
		TokenLBrace, TokenIdent, TokenColon, TokenString, TokenRBrace,
		TokenRParen, TokenEOF,
	}
	require.Len(t, tokens, len(expected))
	for i, tok := range tokens {
		assert.Equal(t, expected[i], tok.Type, "token %d: %s", i, tok.Literal)
	}
}

func TestLexerRelationshipPattern(t *testing.T) {
	tokens := Tokenize("(n)-[:KNOWS]->(m)")
	expected := []TokenType{
		TokenLParen, TokenIdent, TokenRParen,
		TokenDash, TokenLBracket, TokenColon, TokenIdent, TokenRBracket, TokenArrow,
		TokenLParen, TokenIdent, TokenRParen, TokenEOF,
	}
	require.Len(t, tokens, len(expected))
	for i, tok := range tokens {
		assert.Equal(t, expected[i], tok.Type, "token %d: %s", i, tok.Literal)
	}
}

func TestLexerBoolAndNull(t *testing.T) {
	tokens := Tokenize("TRUE FALSE NULL")
	assert.Equal(t, TokenTRUE, tokens[0].Type)
	assert.Equal(t, TokenFALSE, tokens[1].Type)
	assert.Equal(t, TokenNULL, tokens[2].Type)
}

func TestLexerLogicalOps(t *testing.T) {
	tokens := Tokenize("AND OR NOT CONTAINS IN")
	assert.Equal(t, TokenAND, tokens[0].Type)
	assert.Equal(t, TokenOR, tokens[1].Type)
	assert.Equal(t, TokenNOT, tokens[2].Type)
	assert.Equal(t, TokenCONTAINS, tokens[3].Type)
	assert.Equal(t, TokenIN, tokens[4].Type)
}

func TestLexerEmptyInput(t *testing.T) {
	tokens := Tokenize("")
	require.Len(t, tokens, 1)
	assert.Equal(t, TokenEOF, tokens[0].Type)
}
