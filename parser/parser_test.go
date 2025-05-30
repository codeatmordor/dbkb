package parser

import (
	"fmt"
	"strings"
	"testing"

	"inmempg/ast"
	"inmempg/lexer"
	"inmempg/token"
)

func checkParserErrors(t *testing.T, p *Parser) {
	t.Helper()
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}
	t.Errorf("parser has %d errors:", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func TestParseCreateTableStatement(t *testing.T) {
	tests := []struct {
		input             string
		expectedTableName string
		expectedCols      []struct {
			name string
			typ  string
		}
		expectError bool
		errorContains []string
	}{
		{
			input:             "CREATE TABLE users (id INTEGER, name TEXT);",
			expectedTableName: "users",
			expectedCols: []struct {
				name string
				typ  string
			}{{"id", "INTEGER"}, {"name", "TEXT"}},
			expectError: false,
		},
		{
			input:             "CREATE TABLE products (sku TEXT);",
			expectedTableName: "products",
			expectedCols:      []struct{ name string; typ string }{{"sku", "TEXT"}},
			expectError:       false,
		},
		{
			input: "CREATE TABLE orders (order_id INTEGER, customer_id INTEGER, order_date DATETIME)", // No semicolon
			expectedTableName: "orders",
			expectedCols: []struct{ name string; typ string }{
				{"order_id", "INTEGER"},
				{"customer_id", "INTEGER"},
				{"order_date", "DATETIME"},
			},
			expectError: false,
		},
		{
			input: "CREATE users (id INTEGER);", // Missing TABLE keyword
			expectError: true,
			errorContains: []string{"expected next token to be TABLE"},
		},
		{
			input: "CREATE TABLE (id INTEGER);", // Missing table name
			expectError: true,
			errorContains: []string{"expected next token to be IDENT"},
		},
		{
			input: "CREATE TABLE users id INTEGER, name TEXT);", // Missing LPAREN
			expectError: true,
			errorContains: []string{"expected next token to be ("},
		},
		{
			input: "CREATE TABLE users (id INTEGER, name TEXT;", // Missing RPAREN
			expectError: true,
			errorContains: []string{"expected ')' after column definitions"},
		},
		{
			input: "CREATE TABLE users (id, name TEXT);", // Missing type for 'id'
			expectError: true,
			errorContains: []string{"expected column type (identifier) for column id"},
		},
		{
			input: "CREATE TABLE users ();", // Empty column definition
			expectError: true,
			errorContains: []string{"empty column definitions are not allowed"},
		},
		{
			input: "CREATE TABLE users (id INTEGER name TEXT);", // Missing comma
			expectError: true,
			// This error might be tricky, it could manifest as expecting RPAREN or other things
			// depending on parser recovery. For this simple parser, it might try to parse "name" as part of the type for "INTEGER"
			// or fail when it sees "name" after "INTEGER". Let's assume it fails expecting RPAREN or comma.
			// Actual error from current parser: "expected ')' after column definitions, got IDENT" because `nextToken` in `parseColumnDefinitions` moves past `name`
			// then `parseColumnDefinition` for `TEXT` fails or it expects RPAREN
			errorContains: []string{"expected ')' after column definitions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			stmt := p.ParseStatement()

			if tt.expectError {
				if len(p.Errors()) == 0 {
					t.Fatalf("expected parser errors but got none")
				}
				if tt.errorContains != nil {
					for _, errStr := range tt.errorContains {
						found := false
						for _, pErr := range p.Errors() {
							if strings.Contains(pErr, errStr) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("expected error containing %q, but not found in errors: %v", errStr, p.Errors())
						}
					}
				}
				return // Don't check statement if error is expected
			}

			checkParserErrors(t, p)

			if stmt == nil {
				t.Fatalf("ParseStatement() returned nil")
			}
			ctStmt, ok := stmt.(*ast.CreateTableStatement)
			if !ok {
				t.Fatalf("stmt is not *ast.CreateTableStatement. got=%T", stmt)
			}

			if ctStmt.TableName.Value != tt.expectedTableName {
				t.Errorf("TableName.Value not '%s'. got='%s'", tt.expectedTableName, ctStmt.TableName.Value)
			}

			if len(ctStmt.Columns) != len(tt.expectedCols) {
				t.Fatalf("wrong number of columns. expected=%d, got=%d", len(tt.expectedCols), len(ctStmt.Columns))
			}

			for i, expectedCol := range tt.expectedCols {
				actualCol := ctStmt.Columns[i]
				if actualCol.Name.Value != expectedCol.name {
					t.Errorf("column %d name not '%s'. got='%s'", i, expectedCol.name, actualCol.Name.Value)
				}
				if actualCol.DataType.Value != expectedCol.typ {
					t.Errorf("column %d type not '%s'. got='%s'", i, expectedCol.typ, actualCol.DataType.Value)
				}
			}
		})
	}
}

func TestParseInsertStatement(t *testing.T) {
	tests := []struct {
		input             string
		expectedTableName string
		expectedValues    [][]string // string representations of literal values
		expectError       bool
		errorContains     []string
	}{
		{
			input:             "INSERT INTO users VALUES (1, 'Alice');",
			expectedTableName: "users",
			expectedValues:    [][]string{{"1", "Alice"}},
			expectError:       false,
		},
		{
			input:             "INSERT INTO products VALUES ('sku001', 100, 'active');",
			expectedTableName: "products",
			expectedValues:    [][]string{{"sku001", "100", "active"}},
			expectError:       false,
		},
		{
            input: "INSERT INTO orders VALUES (101, 202, '2023-01-15'), (102, 203, '2023-01-16');",
			expectedTableName: "orders",
			expectedValues: [][]string{
				{"101", "202", "2023-01-15"},
				{"102", "203", "2023-01-16"},
			},
			expectError: false,
		},
		{
			input: "INSERT users VALUES (1);", // Missing INTO
			expectError: true,
			errorContains: []string{"expected next token to be INTO"},
		},
		{
			input: "INSERT INTO VALUES (1);", // Missing table name
			expectError: true,
			errorContains: []string{"expected next token to be IDENT"},
		},
		{
			input: "INSERT INTO users (1);", // Missing VALUES
			expectError: true,
			errorContains: []string{"expected next token to be VALUES"},
		},
		{
			input: "INSERT INTO users VALUES 1, 'Alice');", // Missing LPAREN for first tuple
			expectError: true,
			errorContains: []string{"expected '(' to start values list"},
		},
		{
			input: "INSERT INTO users VALUES (1, 'Alice';", // Missing RPAREN for first tuple
			expectError: true,
			errorContains: []string{"expected ')' to end list"},
		},
		{
			input: "INSERT INTO users VALUES (1 'Alice');", // Missing COMMA in tuple
			expectError: true,
			// This error is tricky; current parser might expect RPAREN after the first expression
			errorContains: []string{"expected ')' to end list"},
		},
        {
            input: "INSERT INTO users VALUES (1, ), (2, 'Bob');", // Missing value after comma
            expectError: true,
            errorContains: []string{"unexpected token ) in expression"}, // parseExpression gets ')'
        },
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			stmt := p.ParseStatement()

			if tt.expectError {
				if len(p.Errors()) == 0 {
					t.Fatalf("expected parser errors but got none for input: %s", tt.input)
				}
                 if tt.errorContains != nil {
					for _, errStr := range tt.errorContains {
						found := false
						for _, pErr := range p.Errors() {
							if strings.Contains(pErr, errStr) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("expected error containing %q, but not found in errors: %v", errStr, p.Errors())
						}
					}
				}
				return
			}

			checkParserErrors(t, p)

			if stmt == nil {
				t.Fatalf("ParseStatement() returned nil for input: %s", tt.input)
			}
			isStmt, ok := stmt.(*ast.InsertStatement)
			if !ok {
				t.Fatalf("stmt is not *ast.InsertStatement. got=%T for input: %s", stmt, tt.input)
			}

			if isStmt.TableName.Value != tt.expectedTableName {
				t.Errorf("TableName.Value not '%s'. got='%s'", tt.expectedTableName, isStmt.TableName.Value)
			}

			if len(isStmt.Values) != len(tt.expectedValues) {
				t.Fatalf("wrong number of value rows. expected=%d, got=%d", len(tt.expectedValues), len(isStmt.Values))
			}

			for i, expectedRow := range tt.expectedValues {
				actualRowExprs := isStmt.Values[i]
				if len(actualRowExprs) != len(expectedRow) {
					t.Fatalf("row %d wrong number of values. expected=%d, got=%d", i, len(expectedRow), len(actualRowExprs))
				}
				for j, expectedValStr := range expectedRow {
					lit, ok := actualRowExprs[j].(*ast.LiteralValue)
					if !ok {
						t.Fatalf("row %d, value %d not *ast.LiteralValue. got=%T", i, j, actualRowExprs[j])
					}
					if lit.Value != expectedValStr {
						t.Errorf("row %d, value %d not '%s'. got='%s'", i, j, expectedValStr, lit.Value)
					}
				}
			}
		})
	}
}

func TestParseSelectStatement(t *testing.T) {
	tests := []struct {
		input             string
		expectedTableName string
		expectedCols      []string // "*" for StarSelectColumn, identifier string for others
		expectError       bool
		errorContains     []string
	}{
		{
			input:             "SELECT * FROM users;",
			expectedTableName: "users",
			expectedCols:      []string{"*"},
			expectError:       false,
		},
		{
			input:             "SELECT id, name FROM customers;",
			expectedTableName: "customers",
			expectedCols:      []string{"id", "name"},
			expectError:       false,
		},
		{
			input:             "SELECT id FROM products", // No semicolon
			expectedTableName: "products",
			expectedCols:      []string{"id"},
			expectError:       false,
		},
		{
			input: "SELECT FROM users;", // Missing column list or *
			expectError: true,
			errorContains: []string{"expected identifier or '*' in select list"},
		},
		{
			input: "SELECT id, name users;", // Missing FROM
			expectError: true,
			errorContains: []string{"expected FROM after select list"},
		},
		{
			input: "SELECT id, name FROM ;", // Missing table name
			expectError: true,
			errorContains: []string{"expected next token to be IDENT"},
		},
		{
			input: "SELECT id name FROM users;", // Missing comma
			expectError: true,
			// This error will be "expected FROM after select list, got IDENT" because parseSelectList consumes 'id',
			// then current token becomes 'name'. The loop in parseSelectList expects COMMA or end of list.
			// Since it's not COMMA, it exits, and then parseSelectStatement expects FROM but sees 'name'.
			errorContains: []string{"expected FROM after select list"},
		},
        {
            input: "SELECT id, FROM users;", // Trailing comma before FROM
            expectError: true,
            errorContains: []string{"expected identifier after comma in select list"},
        },

	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			stmt := p.ParseStatement()

			if tt.expectError {
				if len(p.Errors()) == 0 {
					t.Fatalf("expected parser errors but got none for input: %s", tt.input)
				}
                if tt.errorContains != nil {
					for _, errStr := range tt.errorContains {
						found := false
						for _, pErr := range p.Errors() {
							if strings.Contains(pErr, errStr) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("expected error containing %q, but not found in errors: %v", errStr, p.Errors())
						}
					}
				}
				return
			}

			checkParserErrors(t, p)

			if stmt == nil {
				t.Fatalf("ParseStatement() returned nil for input: %s", tt.input)
			}
			ssStmt, ok := stmt.(*ast.SelectStatement)
			if !ok {
				t.Fatalf("stmt is not *ast.SelectStatement. got=%T for input: %s", stmt, tt.input)
			}

			if ssStmt.TableName.Value != tt.expectedTableName {
				t.Errorf("TableName.Value not '%s'. got='%s'", tt.expectedTableName, ssStmt.TableName.Value)
			}

			if len(ssStmt.Columns) != len(tt.expectedCols) {
				t.Fatalf("wrong number of selected columns. expected=%d, got=%d", len(tt.expectedCols), len(ssStmt.Columns))
			}

			for i, expectedColName := range tt.expectedCols {
				actualColExpr := ssStmt.Columns[i]
				if expectedColName == "*" {
					_, isStar := actualColExpr.(*ast.StarSelectColumn)
					if !isStar {
						t.Errorf("column %d not *ast.StarSelectColumn. got=%T", i, actualColExpr)
					}
				} else {
					ident, isIdent := actualColExpr.(*ast.Identifier)
					if !isIdent {
						t.Errorf("column %d not *ast.Identifier. got=%T", i, actualColExpr)
						continue
					}
					if ident.Value != expectedColName {
						t.Errorf("column %d name not '%s'. got='%s'", i, expectedColName, ident.Value)
					}
				}
			}
		})
	}
}
```
