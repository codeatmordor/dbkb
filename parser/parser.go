package parser

import (
	"fmt"
	"strconv"
	"strings"

	"inmempg/ast"
	"inmempg/lexer"
	"inmempg/token"
)

// Operator precedences (no changes needed here for this task)
const (
	_ int = iota
	LOWEST
	EQUALS      // == or != or <>
	LESSGREATER // > or < or >= or <=
	SUM         // + or -
	PRODUCT     // * or /
)

var precedences = map[token.TokenType]int{
	token.EQ:     EQUALS, token.NEQ:    EQUALS, token.ALT_NEQ:EQUALS,
	token.LT:     LESSGREATER, token.GT:     LESSGREATER,
	token.LTE:    LESSGREATER, token.GTE:    LESSGREATER,
	token.PLUS:   SUM, token.MINUS:  SUM,
	token.ASTERISK: PRODUCT, token.SLASH:  PRODUCT,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{ l: l, errors: []string{} }
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifierAsExpression)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(token.FALSE, p.parseBooleanLiteral)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NEQ, p.parseInfixExpression)
	p.registerInfix(token.ALT_NEQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)

	p.nextToken(); p.nextToken()
	return p
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) { p.prefixParseFns[tokenType] = fn }
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) { p.infixParseFns[tokenType] = fn }
func (p *Parser) nextToken() { p.curToken = p.peekToken; p.peekToken = p.l.NextToken() }
func (p *Parser) Errors() []string { return p.errors }
func (p *Parser) addError(msg string) { p.errors = append(p.errors, msg) }
func (p *Parser) peekError(t token.TokenType) {
	p.addError(fmt.Sprintf("expected next token to be %s, got %s instead (literal: %q)",
		t, p.peekToken.Type, p.peekToken.Literal))
}
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) { p.nextToken(); return true }
	p.peekError(t); return false
}
func (p *Parser) curTokenIs(t token.TokenType) bool { return p.curToken.Type == t }
func (p *Parser) peekTokenIs(t token.TokenType) bool { return p.peekToken.Type == t }
func (p *Parser) peekPrecedence() int {
	if pr, ok := precedences[p.peekToken.Type]; ok { return pr }; return LOWEST
}
func (p *Parser) curPrecedence() int {
	if pr, ok := precedences[p.curToken.Type]; ok { return pr }; return LOWEST
}

func (p *Parser) ParseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.CREATE: return p.parseCreateTableStatement()
	case token.INSERT: return p.parseInsertStatement()
	case token.SELECT: return p.parseSelectStatement()
	case token.SAVE: return p.parseSaveStatement()
	case token.LOAD: return p.parseLoadStatement()
	default:
		p.addError(fmt.Sprintf("no parsing function for statement starting with token type %s found", p.curToken.Type))
		return nil
	}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil { p.noPrefixParseFnError(p.curToken.Type); return nil }
	leftExp := prefix()
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() &&
		!p.peekTokenIs(token.RPAREN) && !p.peekTokenIs(token.COMMA) &&
		!p.peekTokenIs(token.FROM) && !p.peekTokenIs(token.EOF) {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil { return leftExp }
		p.nextToken()
		leftExp = infix(leftExp)
	}
	return leftExp
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	p.addError(fmt.Sprintf("no prefix parse function for %s found (literal: %q)", t, p.curToken.Literal))
}
func (p *Parser) parseIdentifierAsExpression() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}
func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.LiteralValue{Token: p.curToken}
	if _, err := strconv.ParseInt(p.curToken.Literal, 0, 64); err != nil {
		p.addError(fmt.Sprintf("could not parse %q as integer: %v", p.curToken.Literal, err)); return nil
	}
	lit.Value = p.curToken.Literal
	return lit
}
func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.LiteralValue{Token: p.curToken, Value: p.curToken.Literal}
}
func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{Token: p.curToken, Operator: p.curToken.Literal, Left: left}
	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)
	return expression
}

func (p *Parser) parseCreateTableStatement() *ast.CreateTableStatement {
	stmt := &ast.CreateTableStatement{Token: p.curToken}
	if !p.expectPeek(token.TABLE) { return nil }
	if !p.expectPeek(token.IDENT) { return nil }
	stmt.TableName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.LPAREN) { return nil }
	p.nextToken()
	stmt.Columns = p.parseColumnDefinitions()
	if !p.curTokenIs(token.RPAREN) {
		p.addError(fmt.Sprintf("expected ')' after column definitions, got %s (literal %q)", p.curToken.Type, p.curToken.Literal))
		return nil
	}
	if p.peekTokenIs(token.SEMICOLON) { p.nextToken() }
	return stmt
}

func (p *Parser) parseColumnDefinitions() []*ast.ColumnDefinition {
	columns := []*ast.ColumnDefinition{}
	if p.curTokenIs(token.RPAREN) { p.addError("empty column definitions are not allowed"); return columns }
	col := p.parseColumnDefinition()
	if col == nil { return nil }
	columns = append(columns, col)
	for p.peekTokenIs(token.COMMA) {
		p.nextToken(); p.nextToken()
		colDef := p.parseColumnDefinition()
		if colDef == nil { return nil }
		columns = append(columns, colDef)
	}
	p.nextToken()
	return columns
}

func (p *Parser) parseColumnDefinition() *ast.ColumnDefinition {
	colDef := &ast.ColumnDefinition{}
	if !p.curTokenIs(token.IDENT) {
		p.addError(fmt.Sprintf("expected column name (identifier), got %s", p.curToken.Type)); return nil
	}
	colDef.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IDENT) {
		p.addError(fmt.Sprintf("expected column type for %s, got %s", colDef.Name.Value, p.peekToken.Type)); return nil
	}
	colDef.DataType = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Handle type parameters for VARCHAR(n) and NUMERIC(p,s)
	dtUpper := strings.ToUpper(colDef.DataType.Value)
	if (dtUpper == "VARCHAR" || dtUpper == "CHAR" || dtUpper == "NUMERIC") && p.peekTokenIs(token.LPAREN) {
		p.nextToken() // Consume LPAREN
		p.nextToken() // Move to the first parameter

		// First parameter (length for VARCHAR, precision for NUMERIC)
		if !p.curTokenIs(token.INT) {
			p.addError(fmt.Sprintf("expected integer parameter for %s, got %s", dtUpper, p.curToken.Type)); return nil
		}
		param1 := p.parseIntegerLiteral()
		if param1 == nil { return nil }
		colDef.TypeParams = append(colDef.TypeParams, param1)

		// Second parameter (scale for NUMERIC)
		if dtUpper == "NUMERIC" && p.peekTokenIs(token.COMMA) {
			p.nextToken() // Consume COMMA
			p.nextToken() // Move to the scale parameter
			if !p.curTokenIs(token.INT) {
				p.addError(fmt.Sprintf("expected integer scale for NUMERIC, got %s", p.curToken.Type)); return nil
			}
			param2 := p.parseIntegerLiteral()
			if param2 == nil { return nil }
			colDef.TypeParams = append(colDef.TypeParams, param2)
		}
		if !p.expectPeek(token.RPAREN) {
			p.addError(fmt.Sprintf("expected ')' after %s parameters, got %s", dtUpper, p.peekToken.Type)); return nil
		}
	}
	return colDef
}

func (p *Parser) parseInsertStatement() *ast.InsertStatement {
	stmt := &ast.InsertStatement{Token: p.curToken}
	if !p.expectPeek(token.INTO) { return nil }
	if !p.expectPeek(token.IDENT) { return nil }
	stmt.TableName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.VALUES) { return nil }
	p.nextToken() 

	valuesList := [][]ast.Expression{}
	if !p.curTokenIs(token.LPAREN) {
		p.addError(fmt.Sprintf("expected '(' to start values list, got %s", p.curToken.Type)); return nil
	}
	for {
		if !p.curTokenIs(token.LPAREN) {
			p.addError(fmt.Sprintf("expected '(' for values tuple, got %s", p.curToken.Type)); return nil
		}
		rowValues, err := p.parseExpressionList(token.RPAREN)
		if err != nil { return nil }
		valuesList = append(valuesList, rowValues)
		if !p.peekTokenIs(token.COMMA) { break }
		p.nextToken(); p.nextToken()
	}
	stmt.Values = valuesList
	if p.peekTokenIs(token.SEMICOLON) { p.nextToken() }
	return stmt
}

func (p *Parser) parseExpressionList(end token.TokenType) ([]ast.Expression, error) {
	list := []ast.Expression{}
	if !p.curTokenIs(token.LPAREN) {
        p.addError(fmt.Sprintf("expected list to start with '(', got %s", p.curToken.Type))
        return nil, fmt.Errorf("list parse error: unexpected start token %s", p.curToken.Type)
    }
	p.nextToken() 
	if p.curTokenIs(end) { p.nextToken(); return list, nil }
	list = append(list, p.parseExpression(LOWEST))
	for p.peekTokenIs(token.COMMA) {
		p.nextToken(); p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}
	if !p.expectPeek(end) {
		return nil, fmt.Errorf("missing closing token %s for expression list", end)
	}
	return list, nil
}

func (p *Parser) parseSelectStatement() *ast.SelectStatement {
	stmt := &ast.SelectStatement{Token: p.curToken}
	p.nextToken()
	cols, err := p.parseSelectListItems()
	if err != nil { return nil }
	stmt.Columns = cols
	if !p.curTokenIs(token.FROM) {
		p.addError(fmt.Sprintf("expected FROM after select list, got %s (literal %q)", p.curToken.Type, p.curToken.Literal))
		return nil
	}
	if !p.expectPeek(token.IDENT) { return nil }
	stmt.TableName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if p.peekTokenIs(token.WHERE) {
		p.nextToken(); p.nextToken()
		stmt.WhereClause = p.parseExpression(LOWEST)
	}
	if p.peekTokenIs(token.SEMICOLON) { p.nextToken() }
	return stmt
}

func (p *Parser) parseSelectListItems() ([]ast.Expression, error) {
	list := []ast.Expression{}
	if p.curTokenIs(token.ASTERISK) && p.peekTokenIs(token.FROM) {
		list = append(list, &ast.StarSelectColumn{Token: p.curToken})
		p.nextToken(); return list, nil
	}
	expr := p.parseExpression(LOWEST)
	if expr == nil { return nil, fmt.Errorf("failed to parse select list item") }
	list = append(list, expr)
	for p.peekTokenIs(token.COMMA) {
		p.nextToken(); p.nextToken()
		expr = p.parseExpression(LOWEST)
		if expr == nil { return nil, fmt.Errorf("failed to parse select list item after comma") }
		list = append(list, expr)
	}
	p.nextToken()
	return list, nil
}

func (p *Parser) parseSaveStatement() ast.Statement {
	stmt := &ast.SaveStatement{Token: p.curToken}
	p.nextToken()
	if !p.curTokenIs(token.STRING) {
		p.addError(fmt.Sprintf("expected string literal for filepath after SAVE, got %s", p.curToken.Type)); return nil
	}
	filePathExpr := p.parseStringLiteral()
	if filePathExpr == nil { return nil }
	stmt.FilePath = filePathExpr.(*ast.LiteralValue)
	if p.peekTokenIs(token.SEMICOLON) { p.nextToken() }
	return stmt
}

func (p *Parser) parseLoadStatement() ast.Statement {
	stmt := &ast.LoadStatement{Token: p.curToken}
	p.nextToken()
	if !p.curTokenIs(token.STRING) {
		p.addError(fmt.Sprintf("expected string literal for filepath after LOAD, got %s", p.curToken.Type)); return nil
	}
	filePathExpr := p.parseStringLiteral()
	if filePathExpr == nil { return nil }
	stmt.FilePath = filePathExpr.(*ast.LiteralValue)
	if p.peekTokenIs(token.SEMICOLON) { p.nextToken() }
	return stmt
}

```
