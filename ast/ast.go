package ast

import (
	"bytes"
	"fmt"
	"strings"

	"inmempg/token"
)

// Node is the base interface for all AST nodes.
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement represents a SQL statement.
type Statement interface {
	Node
	statementNode() // Marker method
}

// Expression represents a value or computation.
type Expression interface {
	Node
	expressionNode() // Marker method
}

// === Basic Concrete Node Types ===

type Identifier struct {
	Token token.Token // token.IDENT
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type LiteralValue struct {
	Token token.Token // e.g., token.STRING, token.INT
	Value string
}

func (lv *LiteralValue) expressionNode()      {}
func (lv *LiteralValue) TokenLiteral() string { return lv.Token.Literal }
func (lv *LiteralValue) String() string {
	if lv.Token.Type == token.STRING {
		return "'" + lv.Value + "'"
	}
	return lv.Value
}

// BooleanLiteral represents a TRUE or FALSE literal.
type BooleanLiteral struct {
	Token token.Token // The TRUE or FALSE token
	Value bool
}

func (bl *BooleanLiteral) expressionNode()      {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) String() string       { return bl.Token.Literal }


type InfixExpression struct {
	Token    token.Token // The operator token, e.g., +
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

// === Statement Node Types ===

type ColumnDefinition struct {
	Name       *Identifier
	DataType   *Identifier  // e.g., "VARCHAR", "BOOLEAN", "INTEGER"
	TypeParams []Expression // For VARCHAR(n), NUMERIC(p,s). Can be *LiteralValue for n.
}

func (cd *ColumnDefinition) TokenLiteral() string { return cd.Name.TokenLiteral() }
func (cd *ColumnDefinition) String() string {
	s := cd.Name.String() + " " + cd.DataType.String()
	if len(cd.TypeParams) > 0 {
		var paramsStr []string
		for _, p := range cd.TypeParams {
			paramsStr = append(paramsStr, p.String())
		}
		s += "(" + strings.Join(paramsStr, ", ") + ")"
	}
	return s
}

type CreateTableStatement struct {
	Token     token.Token
	TableName *Identifier
	Columns   []*ColumnDefinition
}

func (cts *CreateTableStatement) statementNode()       {}
func (cts *CreateTableStatement) TokenLiteral() string { return cts.Token.Literal }
func (cts *CreateTableStatement) String() string {
	var cols []string
	for _, c := range cts.Columns {
		cols = append(cols, c.String())
	}
	return fmt.Sprintf("CREATE TABLE %s (%s)", cts.TableName.String(), strings.Join(cols, ", "))
}

type InsertStatement struct {
	Token     token.Token
	TableName *Identifier
	Values    [][]Expression
}

func (is *InsertStatement) statementNode()       {}
func (is *InsertStatement) TokenLiteral() string { return is.Token.Literal }
func (is *InsertStatement) String() string {
	var rowsStr []string
	for _, row := range is.Values {
		var exprsStr []string
		for _, expr := range row {
			exprsStr = append(exprsStr, expr.String())
		}
		rowsStr = append(rowsStr, "("+strings.Join(exprsStr, ", ")+")")
	}
	return fmt.Sprintf("INSERT INTO %s VALUES %s", is.TableName.String(), strings.Join(rowsStr, ", "))
}

type StarSelectColumn struct {
	Token token.Token // token.ASTERISK
}

func (ssc *StarSelectColumn) expressionNode()      {}
func (ssc *StarSelectColumn) TokenLiteral() string { return ssc.Token.Literal }
func (ssc *StarSelectColumn) String() string       { return "*" }

type SelectStatement struct {
	Token       token.Token
	TableName   *Identifier
	Columns     []Expression
	WhereClause Expression
}

func (ss *SelectStatement) statementNode()       {}
func (ss *SelectStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SelectStatement) String() string {
	var colsStr []string
	for _, col := range ss.Columns {
		colsStr = append(colsStr, col.String())
	}
	base := fmt.Sprintf("SELECT %s FROM %s", strings.Join(colsStr, ", "), ss.TableName.String())
	if ss.WhereClause != nil {
		base += " WHERE " + ss.WhereClause.String()
	}
	return base
}

type SaveStatement struct {
	Token    token.Token
	FilePath *LiteralValue
}

func (ss *SaveStatement) statementNode()       {}
func (ss *SaveStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SaveStatement) String() string {
	return fmt.Sprintf("SAVE %s", ss.FilePath.String())
}

type LoadStatement struct {
	Token    token.Token
	FilePath *LiteralValue
}

func (ls *LoadStatement) statementNode()       {}
func (ls *LoadStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LoadStatement) String() string {
	return fmt.Sprintf("LOAD %s", ls.FilePath.String())
}
```
