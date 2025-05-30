package token

import "strings"

// TokenType is a string representing the type of a token.
type TokenType string

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Literal string
}

// List of token types
const (
	ILLEGAL = "ILLEGAL" // Unknown token/character
	EOF     = "EOF"     // End of File

	// Identifiers & Literals
	IDENT  = "IDENT"  // add, foobar, x, y, ...
	INT    = "INT"    // 1343456
	STRING = "STRING" // "hello world" or 'hello world'
	TRUE   = "TRUE"
	FALSE  = "FALSE"
	// FLOAT token can be added if lexer distinguishes float literals

	// Operators & Delimiters
	LPAREN    = "("
	RPAREN    = ")"
	COMMA     = ","
	SEMICOLON = ";"
	
	// Comparison Operators
	EQ      = "="
	NEQ     = "!=" 
	ALT_NEQ = "<>" 
	LT      = "<"
	GT      = ">"
	LTE     = "<="
	GTE     = ">="

	// Arithmetic Operators
	PLUS     = "+"
	MINUS    = "-"
	ASTERISK = "*" 
	SLASH    = "/"

	// Keywords
	CREATE  = "CREATE"
	TABLE   = "TABLE"
	INSERT  = "INSERT"
	INTO    = "INTO"
	VALUES  = "VALUES"
	SELECT  = "SELECT"
	FROM    = "FROM"
	WHERE   = "WHERE"
	SAVE    = "SAVE"
	LOAD    = "LOAD"
	BOOLEAN = "BOOLEAN"
	VARCHAR = "VARCHAR"
	DATE    = "DATE"
	NUMERIC = "NUMERIC"
)

// keywords maps keyword strings to their TokenType
var keywords = map[string]TokenType{
	"CREATE":  CREATE,
	"TABLE":   TABLE,
	"INSERT":  INSERT,
	"INTO":    INTO,
	"VALUES":  VALUES,
	"SELECT":  SELECT,
	"FROM":    FROM,
	"WHERE":   WHERE,
	"SAVE":    SAVE,
	"LOAD":    LOAD,
	"BOOLEAN": BOOLEAN,
	"VARCHAR": VARCHAR,
	"DATE":    DATE,
	"NUMERIC": NUMERIC,
	"TRUE":    TRUE,
	"FALSE":   FALSE,
}

// LookupIdent checks the keywords map for an identifier's TokenType.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[strings.ToUpper(ident)]; ok {
		return tok
	}
	return IDENT
}
```
