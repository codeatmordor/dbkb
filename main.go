package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"inmempg/ast"
	"inmempg/database"
	"inmempg/lexer"
	"inmempg/parser"
	"inmempg/token"

	"github.com/jeroenrinzema/psql-wire"
	"github.com/jeroenrinzema/psql-wire/pkg/oid"
)

var memDB *database.Database

func main() {
	memDB = database.NewDatabase()
	log.Println("In-memory database initialized.")

	authStrategy := psqlwire.ClearTextPassword(func(ctx context.Context, dbName, username, password string) (context.Context, bool, error) {
		log.Printf("Login attempt: db=%s, user=%s, password=[REDACTED]", dbName, username)
		ctx = context.WithValue(ctx, psqlwire.UsernameKey{}, username)
		ctx = context.WithValue(ctx, psqlwire.DatabaseKey{}, dbName)
		return ctx, true, nil
	})

	opts := []psqlwire.OptionFn{
		psqlwire.SessionAuthStrategy(authStrategy),
	}

	parseFn := func(ctx context.Context, query string) (psqlwire.PreparedStatements, error) {
		log.Printf("Received query from %s: %s", psqlwire.RemoteAddress(ctx), query)

		if strings.TrimSpace(strings.ToLower(query)) == "select 1" {
			cols := psqlwire.Columns{
				psqlwire.Column{Name: "?column?", Oid: oid.Int4, Format: psqlwire.TextFormat},
			}
			stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
				log.Printf("Executing hardcoded 'SELECT 1' for %s", psqlwire.RemoteAddress(execCtx))
				if err := writer.Row([]any{int32(1)}); err != nil {
					return fmt.Errorf("failed to write data row for SELECT 1: %w", err)
				}
				return writer.Complete("SELECT 1")
			}
			preparedStatement := psqlwire.NewStatement(stmtFn, psqlwire.WithColumns(cols))
			return psqlwire.Prepared(preparedStatement), nil
		}
		
		l := lexer.New(query)
		p := parser.New(l)
		astStatement := p.ParseStatement()

		parserErrors := p.Errors()
		if len(parserErrors) > 0 {
			log.Printf("Parser errors for query '%s': %v", query, parserErrors)
			return nil, fmt.Errorf("parser error: %s", strings.Join(parserErrors, "; "))
		}

		if astStatement == nil {
			log.Printf("Parser returned nil statement for query: %s", query)
			return nil, fmt.Errorf("failed to parse query into a valid statement")
		}

		return executeStatement(ctx, memDB, astStatement)
	}

	server, err := psqlwire.NewServer(parseFn, opts...)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	address := "0.0.0.0:5432"
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", address, err)
	}
	defer listener.Close()

	log.Printf("Listening on %s. Use `psql -h localhost -p 5432 -U anyuser -d anydb` to connect.", address)

	if err := server.Serve(listener); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func executeStatement(ctx context.Context, db *database.Database, stmt ast.Statement) (psqlwire.PreparedStatements, error) {
	remoteAddr := psqlwire.RemoteAddress(ctx)

	switch s := stmt.(type) {
	case *ast.CreateTableStatement:
		log.Printf("Executing CREATE TABLE statement for %s: %s", remoteAddr, s.TableName.Value)
		var columnSchemas []database.ColumnSchema
		for _, astColDef := range s.Columns {
			colType, err := database.StringToBasicType(astColDef.DataType.Value)
			if err != nil {
				log.Printf("Error converting data type for column %s in table %s: %v", astColDef.Name.Value, s.TableName.Value, err)
				return nil, fmt.Errorf("invalid data type for column %q: %w", astColDef.Name.Value, err)
			}
			columnSchemas = append(columnSchemas, database.ColumnSchema{
				Name: astColDef.Name.Value,
				Type: colType,
			})
		}
		err := db.CreateTable(s.TableName.Value, columnSchemas)
		if err != nil {
			log.Printf("Error creating table %s for %s: %v", s.TableName.Value, remoteAddr, err)
			return nil, err
		}
		log.Printf("Successfully created table %s for %s", s.TableName.Value, remoteAddr)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			return writer.Complete("CREATE TABLE")
		}
		preparedStatement := psqlwire.NewStatement(stmtFn)
		return psqlwire.Prepared(preparedStatement), nil

	case *ast.InsertStatement:
		log.Printf("Executing INSERT INTO statement for %s: %s", remoteAddr, s.TableName.Value)
		lowerTableName := strings.ToLower(s.TableName.Value)
		table, exists := db.Tables[lowerTableName]
		if !exists {
			return nil, fmt.Errorf("table %q does not exist", s.TableName.Value)
		}
		var rowsAffectedCount int64 = 0
		for _, astRowValues := range s.Values {
			if len(astRowValues) != len(table.Schema.Columns) {
				return nil, fmt.Errorf("column count mismatch: table %q has %d columns, but %d values were supplied",
					s.TableName.Value, len(table.Schema.Columns), len(astRowValues))
			}
			newGoRow := make(database.Row, len(astRowValues))
			for i, astExpr := range astRowValues {
				literal, ok := astExpr.(*ast.LiteralValue)
				if !ok {
					return nil, fmt.Errorf("INSERT values must be literals at this stage (got %T)", astExpr)
				}
				switch literal.Token.Type {
				case token.INT:
					val, err := strconv.ParseInt(literal.Value, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("invalid integer literal %q: %w", literal.Value, err)
					}
					newGoRow[i] = val
				case token.STRING:
					newGoRow[i] = literal.Value
				default:
					return nil, fmt.Errorf("unsupported literal type %s in INSERT statement", literal.Token.Type)
				}
			}
			err := db.InsertRow(s.TableName.Value, newGoRow)
			if err != nil {
				log.Printf("Error inserting row into table %s for %s: %v", s.TableName.Value, remoteAddr, err)
				return nil, err
			}
			rowsAffectedCount++
		}
		log.Printf("Successfully inserted %d row(s) into table %s for %s", rowsAffectedCount, s.TableName.Value, remoteAddr)
		commandTag := fmt.Sprintf("INSERT 0 %d", rowsAffectedCount)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			return writer.Complete(commandTag)
		}
		preparedStatement := psqlwire.NewStatement(stmtFn)
		return psqlwire.Prepared(preparedStatement), nil

	case *ast.SelectStatement:
		log.Printf("Executing SELECT statement for %s: %s", remoteAddr, s.TableName.Value)
		
		var selectedColumnNames []string
		if len(s.Columns) == 1 {
			if _, ok := s.Columns[0].(*ast.StarSelectColumn); ok {
				selectedColumnNames = []string{"*"}
			}
		}
		if len(selectedColumnNames) == 0 { // Not a SELECT * or already processed as such
			selectedColumnNames = make([]string, 0, len(s.Columns))
			for _, colExpr := range s.Columns {
				ident, ok := colExpr.(*ast.Identifier)
				if !ok {
					return nil, fmt.Errorf("unsupported select item: expected identifier or '*', got %T", colExpr)
				}
				selectedColumnNames = append(selectedColumnNames, ident.Value)
			}
		}

		resultSchema, resultRows, err := db.SelectRows(s.TableName.Value, selectedColumnNames)
		if err != nil {
			log.Printf("Error selecting rows from table %s for %s: %v", s.TableName.Value, remoteAddr, err)
			return nil, err
		}

		// Prepare RowDescription
		psqlwireCols := make(psqlwire.Columns, len(resultSchema.Columns))
		for i, colSchema := range resultSchema.Columns {
			pgOid := database.BasicTypeToPgOid(colSchema.Type)
			psqlwireCols[i] = psqlwire.Column{
				Name:   colSchema.Name, // Use original casing from schema
				Oid:    pgOid,
				Format: psqlwire.TextFormat, // For simplicity, all columns as text
			}
		}

		// Prepare PreparedStatementFn to send rows
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			var rowCount int64 = 0
			for _, dbRow := range resultRows {
				rowDataForWire := make([]interface{}, len(dbRow))
				for i, val := range dbRow {
					if val == nil {
						rowDataForWire[i] = nil // Pass nil directly
					} else {
						// Convert all values to string for TextFormat, as psql-wire expects string or []byte for text.
						// More sophisticated type handling might be needed for binary format or specific client expectations.
						switch v := val.(type) {
						case int64:
							rowDataForWire[i] = strconv.FormatInt(v, 10)
						case float64:
							rowDataForWire[i] = strconv.FormatFloat(v, 'f', -1, 64)
						case string:
							rowDataForWire[i] = v
						case []byte:
							rowDataForWire[i] = string(v) // Or handle as bytea appropriately if format was binary
						default:
							rowDataForWire[i] = fmt.Sprintf("%v", v)
						}
					}
				}
				if err := writer.Row(rowDataForWire); err != nil {
					log.Printf("Error writing data row for SELECT on %s for %s: %v", s.TableName.Value, remoteAddr, err)
					return fmt.Errorf("failed to write data row: %w", err)
				}
				rowCount++
			}
			log.Printf("Successfully sent %d row(s) for SELECT on table %s for %s", rowCount, s.TableName.Value, remoteAddr)
			return writer.Complete(fmt.Sprintf("SELECT %d", rowCount))
		}

		preparedStatement := psqlwire.NewStatement(stmtFn, psqlwire.WithColumns(psqlwireCols))
		return psqlwire.Prepared(preparedStatement), nil

	default:
		log.Printf("Unsupported AST statement type for execution: %T for %s", s, remoteAddr)
		return nil, fmt.Errorf("unsupported statement type for execution: %T", s)
	}
}
```
