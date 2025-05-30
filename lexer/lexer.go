package lexer

import (
	"strings"
	"unicode"

	"inmempg/token" // Assuming inmempg is the module name
)

// Lexer holds the input string, current position, and next read position.
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
}

// New creates a new Lexer.
func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar() // Initialize l.ch, l.position, and l.readPosition
	return l
}

// readChar gives us the next character and advances our position in the input string.
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // ASCII code for "NUL" character, signifies EOF or not read anything yet
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

// peekChar returns the next character without consuming it.
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// NextToken determines and returns the next token from the input.
func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()

	switch l.ch {
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
	case '\'': // Start of a string literal
		tok.Type = token.STRING
		tok.Literal = l.readString()
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		if isLetter(l.ch) { // Identifiers or keywords
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal) // Check if it's a keyword
			return tok                               // Early return because readIdentifier advances pointers
		} else if isDigit(l.ch) { // Numbers
			tok.Type = token.INT
			tok.Literal = l.readNumber()
			return tok // Early return because readNumber advances pointers
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar() // Advance to the next character
	return tok
}

// skipWhitespace consumes all subsequent whitespace characters.
func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(rune(l.ch)) {
		l.readChar()
	}
}

// readIdentifier reads a sequence of letters/digits/underscores as an identifier.
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a sequence of digits as a number.
func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readString reads a string literal enclosed in single quotes.
// It handles escaped single quotes ('') inside the string.
func (l *Lexer) readString() string {
	var sb strings.Builder
	l.readChar() // Consume the opening quote

	for {
		if l.ch == 0 { // EOF before closing quote
			// Consider this an illegal/unterminated string
			// For now, returning what we have, but error handling could be added
			return sb.String()
		}
		if l.ch == '\'' {
			if l.peekChar() == '\'' { // Escaped single quote ('')
				l.readChar() // Consume the first quote of the pair
				sb.WriteByte(l.ch) // Write the second quote
				l.readChar() // Consume the second quote
			} else { // End of string
				l.readChar() // Consume the closing quote
				break
			}
		} else {
			sb.WriteByte(l.ch)
			l.readChar()
		}
	}
	return sb.String()
}

// isLetter checks if the character is a letter or underscore (common for identifiers).
func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ch == '_'
}

// isDigit checks if the character is a digit.
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// newToken is a helper function to create a new Token.
func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}
```
