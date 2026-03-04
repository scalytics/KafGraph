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

// TokenType identifies the kind of lexical token.
type TokenType int

// Cypher token types used by the lexer.
const (
	// TokenIdent represents an identifier literal.
	TokenIdent TokenType = iota
	TokenString
	TokenInt
	TokenFloat
	TokenParam

	// TokenMATCH represents the MATCH keyword.
	TokenMATCH
	TokenWHERE
	TokenRETURN
	TokenCREATE
	TokenMERGE
	TokenSET
	TokenDELETE
	TokenORDER
	TokenBY
	TokenLIMIT
	TokenSKIP
	TokenAS
	TokenAND
	TokenOR
	TokenNOT
	TokenCONTAINS
	TokenIN
	TokenCALL
	TokenYIELD
	TokenASC
	TokenDESC
	TokenCOUNT
	TokenWITH
	TokenUNWIND
	TokenTRUE
	TokenFALSE
	TokenNULL

	// TokenLParen represents the '(' symbol.
	TokenLParen
	TokenRParen
	TokenLBracket
	TokenRBracket
	TokenLBrace
	TokenRBrace
	TokenColon
	TokenComma
	TokenDot
	TokenStar
	TokenEQ
	TokenNEQ
	TokenLT
	TokenGT
	TokenLTE
	TokenGTE
	TokenDash
	TokenArrow
	TokenLArrow
	TokenPipe

	TokenEOF
	TokenIllegal
)

// Token represents a single lexical token.
type Token struct {
	Type    TokenType
	Literal string
	Pos     int
}

// keywords maps uppercase keyword strings to their token types.
var keywords = map[string]TokenType{
	"MATCH":    TokenMATCH,
	"WHERE":    TokenWHERE,
	"RETURN":   TokenRETURN,
	"CREATE":   TokenCREATE,
	"MERGE":    TokenMERGE,
	"SET":      TokenSET,
	"DELETE":   TokenDELETE,
	"ORDER":    TokenORDER,
	"BY":       TokenBY,
	"LIMIT":    TokenLIMIT,
	"SKIP":     TokenSKIP,
	"AS":       TokenAS,
	"AND":      TokenAND,
	"OR":       TokenOR,
	"NOT":      TokenNOT,
	"CONTAINS": TokenCONTAINS,
	"IN":       TokenIN,
	"CALL":     TokenCALL,
	"YIELD":    TokenYIELD,
	"ASC":      TokenASC,
	"DESC":     TokenDESC,
	"COUNT":    TokenCOUNT,
	"WITH":     TokenWITH,
	"UNWIND":   TokenUNWIND,
	"TRUE":     TokenTRUE,
	"FALSE":    TokenFALSE,
	"NULL":     TokenNULL,
}

// LookupIdent returns the keyword token type for ident, or TokenIdent.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokenIdent
}
