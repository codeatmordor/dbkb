package parser

import (
	"fmt"
	"strconv" // For parsing integer literals if needed, though AST stores literal as string

	"inmempg/ast"
	"inmempg/lexer"
	"inmempg/token"
)

// Parser holds the lexer, current and peek tokens, and any parsing errors.
type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token
}

// New creates a new Parser.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}
	// Read two tokens, so curToken and peekToken are both set.
	p.nextToken()
	p.nextToken()
	return p
}

// nextToken advances the current and peek tokens.
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// Errors returns the list of parsing errors.
func (p *Parser) Errors() []string {
	return p.errors
}

// addError records a parsing error.
func (p *Parser) addError(msg string) {
	p.errors = append(p.errors, msg)
}

// peekError adds an error if the peekToken type doesn't match expected.
func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t, p.peekToken.Type)
	p.addError(msg)
}

// expectPeek checks peekToken type. If it matches, advances tokens and returns true.
// Otherwise, adds a peekError and returns false.
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

// curTokenIs checks the type of the current token.
func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// peekTokenIs checks the type of the peek token.
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// ParseStatement is the main entry point for parsing a single SQL statement.
func (p *Parser) ParseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.CREATE:
		return p.parseCreateTableStatement()
	case token.INSERT:
		return p.parseInsertStatement()
	case token.SELECT:
		return p.parseSelectStatement()
	default:
		msg := fmt.Sprintf("no parsing function for token type %s found", p.curToken.Type)
		p.addError(msg)
		return nil
	}
}

// --- Identifier Parsing ---
func (p *Parser) parseIdentifier() *ast.Identifier {
	if !p.curTokenIs(token.IDENT) {
		p.addError(fmt.Sprintf("expected identifier, got %s (%q)", p.curToken.Type, p.curToken.Literal))
		return nil
	}
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// --- CREATE TABLE Statement Parsing ---
// CREATE TABLE <TableName> ( <ColumnDefinitions> );
func (p *Parser) parseCreateTableStatement() *ast.CreateTableStatement {
	stmt := &ast.CreateTableStatement{Token: p.curToken}

	if !p.expectPeek(token.TABLE) {
		return nil
	}
	if !p.expectPeek(token.IDENT) { // TableName
		return nil
	}
	stmt.TableName = p.parseIdentifier() // curToken is now IDENT

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken() // Consume LPAREN

	stmt.Columns = p.parseColumnDefinitions()

	if !p.curTokenIs(token.RPAREN) { // Should be RPAREN after column definitions
		p.addError(fmt.Sprintf("expected ')' after column definitions, got %s", p.curToken.Type))
		return nil
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseColumnDefinitions() []*ast.ColumnDefinition {
	columns := []*ast.ColumnDefinition{}

	if p.curTokenIs(token.RPAREN) { // Empty column list
		p.addError("empty column definitions are not allowed")
		return columns // or nil, depending on how strict
	}

	col := p.parseColumnDefinition()
	if col == nil {
		return nil // Error already added by parseColumnDefinition
	}
	columns = append(columns, col)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume COMMA
		p.nextToken() // Move to the next identifier (column name)
		colDef := p.parseColumnDefinition()
		if colDef == nil {
			return nil // Error already added
		}
		columns = append(columns, colDef)
	}
	p.nextToken() // Move past the last column definition element (type) to prepare for RPAREN check
	return columns
}

// <ColName> <ColType>
func (p *Parser) parseColumnDefinition() *ast.ColumnDefinition {
	colDef := &ast.ColumnDefinition{}

	if !p.curTokenIs(token.IDENT) {
		p.addError(fmt.Sprintf("expected column name (identifier), got %s", p.curToken.Type))
		return nil
	}
	colDef.Name = p.parseIdentifier() // curToken is IDENT

	if !p.expectPeek(token.IDENT) { // DataType
		p.addError(fmt.Sprintf("expected column type (identifier) for column %s, got %s", colDef.Name.Value, p.peekToken.Type))
		return nil
	}
	colDef.DataType = p.parseIdentifier() // curToken is now IDENT (DataType)

	return colDef
}

// --- INSERT INTO Statement Parsing ---
// INSERT INTO <TableName> VALUES ( <ValueList> ), ( <ValueList> ), ... ;
func (p *Parser) parseInsertStatement() *ast.InsertStatement {
	stmt := &ast.InsertStatement{Token: p.curToken}

	if !p.expectPeek(token.INTO) {
		return nil
	}
	if !p.expectPeek(token.IDENT) { // TableName
		return nil
	}
	stmt.TableName = p.parseIdentifier() // curToken is IDENT

	if !p.expectPeek(token.VALUES) {
		return nil
	}
	p.nextToken() // Consume VALUES

	// Parse list of value tuples
	valuesList := [][]ast.Expression{}
	if !p.curTokenIs(token.LPAREN) {
		p.addError(fmt.Sprintf("expected '(' to start values list, got %s", p.curToken.Type))
		return nil
	}
	
	rowValues, err := p.parseExpressionList(token.RPAREN)
	if err != nil {
		// error already added by parseExpressionList or its children
		return nil
	}
	valuesList = append(valuesList, rowValues)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume COMMA separating value tuples
		p.nextToken() // Consume the LPAREN of the next tuple
		if !p.curTokenIs(token.LPAREN) {
			p.addError(fmt.Sprintf("expected '(' for next values tuple, got %s", p.curToken.Type))
			return nil
		}
		rowValues, err := p.parseExpressionList(token.RPAREN)
		if err != nil {
			return nil
		}
		valuesList = append(valuesList, rowValues)
	}
	stmt.Values = valuesList

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// parseExpressionList parses a list of expressions until an 'end' token type is encountered.
func (p *Parser) parseExpressionList(end token.TokenType) ([]ast.Expression, error) {
	list := []ast.Expression{}

	if !p.curTokenIs(token.LPAREN) { // Should start with LPAREN (already consumed by caller for first item)
		// this is actually an error, caller should ensure curToken is LPAREN
		p.addError(fmt.Sprintf("expected list to start with '(', got %s", p.curToken.Type))
		return nil, fmt.Errorf("list must start with LPAREN")
	}
	p.nextToken() // Consume LPAREN

	if p.curTokenIs(end) { // Empty list like ()
		p.nextToken() // Consume the 'end' token
		return list, nil
	}

	expr := p.parseExpression()
	if expr == nil {
		return nil, fmt.Errorf("failed to parse expression in list")
	}
	list = append(list, expr)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume COMMA
		p.nextToken() // Move to the next expression
		expr := p.parseExpression()
		if expr == nil {
			return nil, fmt.Errorf("failed to parse expression in list after comma")
		}
		list = append(list, expr)
	}

	if !p.expectPeek(end) { // Expect the 'end' token (e.g., RPAREN)
		return nil, fmt.Errorf("expected %s to end list, got %s", end, p.peekToken.Type)
	}
	// expectPeek consumes the 'end' token if successful

	return list, nil
}

// parseExpression is a rudimentary expression parser.
// For now, it only handles INT and STRING literals.
func (p *Parser) parseExpression() ast.Expression {
	switch p.curToken.Type {
	case token.INT:
		// Here you could add validation for the integer string if needed
		// For example, strconv.ParseInt(p.curToken.Literal, 10, 64)
		// But AST stores the literal string.
		return &ast.LiteralValue{Token: p.curToken, Value: p.curToken.Literal}
	case token.STRING:
		return &ast.LiteralValue{Token: p.curToken, Value: p.curToken.Literal}
	case token.IDENT: // Could be a column name in other contexts, but for VALUES, it's unusual unless it's an enum or special const.
		// For current INSERT VALUES, we expect literals. Treating IDENT as error here for VALUES context.
		p.addError(fmt.Sprintf("unexpected identifier %q in expression, expected literal for INSERT VALUES", p.curToken.Literal))
		return nil
	default:
		p.addError(fmt.Sprintf("unexpected token %s (%q) in expression", p.curToken.Type, p.curToken.Literal))
		return nil
	}
}

// --- SELECT Statement Parsing ---
// SELECT <ColumnList> FROM <TableName>;
func (p *Parser) parseSelectStatement() *ast.SelectStatement {
	stmt := &ast.SelectStatement{Token: p.curToken}

	p.nextToken() // Consume SELECT

	cols, err := p.parseSelectList()
	if err != nil {
		// error already added
		return nil
	}
	stmt.Columns = cols

	if !p.curTokenIs(token.FROM) { // After select list, current token should be FROM
		p.addError(fmt.Sprintf("expected FROM after select list, got %s", p.curToken.Type))
		return nil
	}

	if !p.expectPeek(token.IDENT) { // TableName
		return nil
	}
	stmt.TableName = p.parseIdentifier() // curToken is IDENT

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseSelectList() ([]ast.Expression, error) {
	list := []ast.Expression{}

	if p.curTokenIs(token.ASTERISK) {
		list = append(list, &ast.StarSelectColumn{Token: p.curToken})
		p.nextToken() // Consume ASTERISK
		return list, nil
	}

	// Parse first identifier
	if !p.curTokenIs(token.IDENT) {
		p.addError(fmt.Sprintf("expected identifier or '*' in select list, got %s", p.curToken.Type))
		return nil, fmt.Errorf("invalid select list")
	}
	list = append(list, p.parseIdentifier()) // curToken is IDENT

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // Consume COMMA
		p.nextToken() // Move to next IDENT
		if !p.curTokenIs(token.IDENT) {
			p.addError(fmt.Sprintf("expected identifier after comma in select list, got %s", p.curToken.Type))
			return nil, fmt.Errorf("invalid select list")
		}
		list = append(list, p.parseIdentifier())
	}
	p.nextToken() // Consume the last identifier in the list to prepare for FROM check

	return list, nil
}
```
