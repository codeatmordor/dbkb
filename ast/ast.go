package ast

import (
	"inmempg/token" // Assuming inmempg is the module name
)

// Node is the base interface for all AST nodes.
type Node interface {
	TokenLiteral() string // Returns the literal value of the token this node is associated with
	String() string       // For debugging and testing
}

// Statement represents a SQL statement (e.g., CREATE TABLE, INSERT, SELECT).
// All statement nodes implement this interface.
type Statement interface {
	Node
	statementNode() // Marker method
}

// Expression represents a value or computation (e.g., a literal, an identifier, an arithmetic operation).
// All expression nodes implement this interface.
type Expression interface {
	Node
	expressionNode() // Marker method
}

// === Basic Concrete Node Types ===

// Identifier represents an identifier (e.g., table name, column name).
type Identifier struct {
	Token token.Token // The token.IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

// LiteralValue represents a literal value like a string or a number.
// This is a basic form; in a more complex parser, you'd have distinct
// StringLiteral, IntegerLiteral, etc., implementing Expression.
type LiteralValue struct {
	Token token.Token // The token (e.g., token.STRING, token.INT)
	Value string      // The actual value
}

func (lv *LiteralValue) expressionNode()      {}
func (lv *LiteralValue) TokenLiteral() string { return lv.Token.Literal }
func (lv *LiteralValue) String() string       { return lv.Token.Literal } // Or lv.Value, depending on desired string output

// === Statement Node Types ===

// ColumnDefinition defines a column in a CREATE TABLE statement.
type ColumnDefinition struct {
	Name     *Identifier // Column name
	DataType *Identifier // Data type (e.g., "TEXT", "INTEGER") - represented as an Identifier for now
}

func (cd *ColumnDefinition) TokenLiteral() string { return cd.Name.TokenLiteral() }
func (cd *ColumnDefinition) String() string {
	return cd.Name.String() + " " + cd.DataType.String()
}

// CreateTableStatement represents a CREATE TABLE statement.
type CreateTableStatement struct {
	Token     token.Token // The 'CREATE' token
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
	return "CREATE TABLE " + cts.TableName.String() + " (" + strings.Join(cols, ", ") + ")"
}

// InsertStatement represents an INSERT INTO ... VALUES ... statement.
type InsertStatement struct {
	Token     token.Token // The 'INSERT' token
	TableName *Identifier
	// Columns   []*Identifier // Optional: list of columns to insert into, not handled in this MVP
	Values    [][]Expression // A list of rows (tuples) to insert, each row is a list of expressions (literals for now)
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
	return "INSERT INTO " + is.TableName.String() + " VALUES " + strings.Join(rowsStr, ", ")
}


// SelectColumn represents an item in a SELECT clause (e.g., a column name or '*').
// For this MVP, we'll simplify. A more robust AST would have specific types.
// We can use an Expression for named columns, and a special struct for '*'.

// StarSelectColumn represents a '*' in a SELECT statement.
type StarSelectColumn struct {
	Token token.Token // The token.ASTERISK token
}
func (ssc *StarSelectColumn) expressionNode() {} // Can act as an expression in some contexts
func (ssc *StarSelectColumn) TokenLiteral() string { return ssc.Token.Literal }
func (ssc *StarSelectColumn) String() string       { return "*" }


// SelectStatement represents a SELECT statement.
// This is a very simplified version for "SELECT <columns> FROM <table>".
type SelectStatement struct {
	Token     token.Token // The 'SELECT' token
	TableName *Identifier
	Columns   []Expression // List of columns to select. Can be *Identifier or specific *StarSelectColumn.
}

func (ss *SelectStatement) statementNode()       {}
func (ss *SelectStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SelectStatement) String() string {
	var colsStr []string
	for _, col := range ss.Columns {
		colsStr = append(colsStr, col.String())
	}
	return "SELECT " + strings.Join(colsStr, ", ") + " FROM " + ss.TableName.String()
}

// Helper function for string join, not part of AST but used in String() methods.
// Placed here to avoid import cycle if ast.go itself needed it from another package.
// Alternatively, make String() methods on structs more self-contained or use fmt.Sprintf.
// For now, ensuring `strings` is imported in the context where these structs are used.
// The String() methods in this file already use `strings.Join`.
```
