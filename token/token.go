package token

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

	// Operators & Delimiters
	LPAREN   = "("
	RPAREN   = ")"
	COMMA    = ","
	SEMICOLON = ";"
	ASTERISK = "*"

	// Keywords
	CREATE = "CREATE"
	TABLE  = "TABLE"
	INSERT = "INSERT"
	INTO   = "INTO"
	VALUES = "VALUES"
	SELECT = "SELECT"
	FROM   = "FROM"
)

// keywords maps keyword strings to their TokenType
var keywords = map[string]TokenType{
	"CREATE": CREATE,
	"TABLE":  TABLE,
	"INSERT": INSERT,
	"INTO":   INTO,
	"VALUES": VALUES,
	"SELECT": SELECT,
	"FROM":   FROM,
}

// LookupIdent checks the keywords map for an identifier's TokenType.
// If it's not a keyword, it returns IDENT.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[strings.ToUpper(ident)]; ok { // Case-insensitive keywords
		return tok
	}
	return IDENT
}
```
