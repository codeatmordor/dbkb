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
	case '+':
		tok = newToken(token.PLUS, l.ch)
	case '-':
		tok = newToken(token.MINUS, l.ch)
	case '/':
		tok = newToken(token.SLASH, l.ch)
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
	case '=':
		tok = newToken(token.EQ, l.ch)
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar() // consume the '='
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NEQ, Literal: literal}
		} else {
			// '!' by itself is not a standard SQL operator we're supporting here.
			tok = newToken(token.ILLEGAL, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar() // consume the '='
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.LTE, Literal: literal}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar() // consume the '>'
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.ALT_NEQ, Literal: literal}
		} else {
			tok = newToken(token.LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar() // consume the '='
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.GTE, Literal: literal}
		} else {
			tok = newToken(token.GT, l.ch)
		}
	case '\'':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		// after readString, l.ch is the closing quote.
		// The l.readChar() at the end of NextToken will consume it.
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok // readIdentifier already called readChar, so we return early
		} else if isDigit(l.ch) {
			tok.Type = token.INT
			tok.Literal = l.readNumber()
			return tok // readNumber already called readChar, so we return early
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar() // Move to the next character for the *next* call to NextToken
	return tok
}

// skipWhitespace consumes all subsequent whitespace characters.
func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(rune(l.ch)) {
		l.readChar()
	}
}

// readIdentifier reads a sequence of letters/digits/underscores as an identifier.
// It advances the lexer's position to the character *after* the identifier.
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) { // Subsequent characters can also be digits
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a sequence of digits as a number.
// It advances the lexer's position to the character *after* the number.
func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readString reads a string literal enclosed in single quotes.
// It handles escaped single quotes ('') inside the string.
// It returns the content *between* the quotes.
// Assumes l.ch is the opening single quote when called.
// Leaves l.ch as the closing single quote.
func (l *Lexer) readString() string {
	var sb strings.Builder
	// l.ch is the opening quote. Consume it to start reading the content.
	// No, NextToken's case already identified it. We need to read *past* it.
	// The char read by the main loop *before* this switch was the opening quote.
	// So, l.readChar() here will get the first char *of the content*.

	for {
		l.readChar() // Read next char for content or closing quote
		if l.ch == 0 {
			// Unterminated string, error. For now, return what we have.
			// A more robust lexer might return an ILLEGAL token or store an error.
			return sb.String()
		}
		if l.ch == '\'' {
			if l.peekChar() == '\'' { // Escaped single quote ('')
				l.readChar()       // Consume the first quote of the 'pair' (which is current l.ch)
				sb.WriteByte(l.ch) // Write the actual single quote (which was peekChar(), now current l.ch)
				// Loop will continue, and next readChar will move past the second quote.
			} else {
				// This is the closing quote. Break loop. l.ch is this closing quote.
				break
			}
		} else {
			sb.WriteByte(l.ch)
		}
	}
	return sb.String()
}

// isLetter checks if the character is a letter (a-z, A-Z) or underscore.
func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ch == '_'
}

// isDigit checks if the character is a digit (0-9).
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// newToken is a helper function to create a new Token from a single byte.
func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}
```
