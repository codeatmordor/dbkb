package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time" // For time.Parse in INSERT

	"inmempg/ast"
	"inmempg/database"
	"inmempg/evaluator"
	"inmempg/lexer"
	"inmempg/parser"
	"inmempg/token"

	"github.com/jeroenrinzema/psql-wire"
	"github.com/jeroenrinzema/psql-wire/pkg/oid"
)

var memDB *database.Database
const DATE_FORMAT_INPUT = "2006-01-02" // For parsing date strings from user
const DATE_FORMAT_OUTPUT = "2006-01-02" // For displaying dates

func main() {
	memDB = database.NewDatabase()
	log.Println("In-memory database initialized.")

	authStrategy := psqlwire.ClearTextPassword(func(ctx context.Context, dbName, username, password string) (context.Context, bool, error) {
		log.Printf("Login attempt: db=%s, user=%s, password=[REDACTED]", dbName, username)
		ctx = context.WithValue(ctx, psqlwire.UsernameKey{}, username)
		ctx = context.WithValue(ctx, psqlwire.DatabaseKey{}, dbName)
		return ctx, true, nil
	})

	opts := []psqlwire.OptionFn{ psqlwire.SessionAuthStrategy(authStrategy) }
	parseFn := func(ctx context.Context, query string) (psqlwire.PreparedStatements, error) {
		log.Printf("Received query from %s: %s", psqlwire.RemoteAddress(ctx), query)
		if strings.TrimSpace(strings.ToLower(query)) == "select 1" { /* ... SELECT 1 handling ... */
			cols := psqlwire.Columns{psqlwire.Column{Name: "?column?", Oid: oid.Int4, Format: psqlwire.TextFormat}}
			stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
				if err := writer.Row([]any{int32(1)}); err != nil { return fmt.Errorf("failed to write data row for SELECT 1: %w", err) }
				return writer.Complete("SELECT 1")
			}
			return psqlwire.Prepared(psqlwire.NewStatement(stmtFn, psqlwire.WithColumns(cols))), nil
		}
		l := lexer.New(query); p := parser.New(l); astStatement := p.ParseStatement()
		parserErrors := p.Errors()
		if len(parserErrors) > 0 {
			log.Printf("Parser errors for query '%s': %v", query, parserErrors)
			return nil, fmt.Errorf("parser error: %s", strings.Join(parserErrors, "; "))
		}
		if astStatement == nil {
			log.Printf("Parser returned nil statement for query: %s", query)
			return nil, fmt.Errorf("failed to parse query into a valid statement (nil AST)")
		}
		return executeStatement(ctx, memDB, astStatement)
	}
	server, err := psqlwire.NewServer(parseFn, opts...)
	if err != nil { log.Fatalf("Failed to create server: %v", err) }
	address := "0.0.0.0:5432"; listener, err := net.Listen("tcp", address)
	if err != nil { log.Fatalf("Failed to listen on %s: %v", address, err) }
	defer listener.Close()
	log.Printf("Listening on %s. Use `psql -h localhost -p 5432 -U anyuser -d anydb` to connect.", address)
	if err := server.Serve(listener); err != nil { log.Fatalf("Server failed: %v", err) }
}

func executeStatement(ctx context.Context, db *database.Database, stmt ast.Statement) (psqlwire.PreparedStatements, error) {
	remoteAddr := psqlwire.RemoteAddress(ctx)
	switch s := stmt.(type) {
	case *ast.CreateTableStatement:
		log.Printf("Executing CREATE TABLE statement for %s: %s", remoteAddr, s.TableName.Value)
		var columnSchemas []database.ColumnSchema
		for _, astColDef := range s.Columns {
			var params []string
			for _, pExpr := range astColDef.TypeParams {
				if pLit, ok := pExpr.(*ast.LiteralValue); ok { // Expecting LiteralValue for params
					params = append(params, pLit.Value)
				} else {
					return nil, fmt.Errorf("invalid type parameter for %s on column %q: expected literal, got %T",
						astColDef.DataType.Value, astColDef.Name.Value, pExpr)
				}
			}
			
			// StringToBasicTypeExtended expects specific number of params based on type
			var typeParam1, typeParam2 string
			if len(params) > 0 { typeParam1 = params[0] }
			if len(params) > 1 { typeParam2 = params[1] }
			
			// This is a simplified way to pass params. Real StringToBasicTypeExtended might need more structure.
			// For now, let's assume StringToBasicTypeExtended can handle it or make it simpler.
			// The database/database.go StringToBasicTypeExtended was modified to accept *ast.Identifier and []ast.Expression
			// So we pass them directly.
			colType, colParams, err := database.StringToBasicTypeExtended(astColDef.DataType, astColDef.TypeParams)

			if err != nil {
				log.Printf("Error converting data type for column %s in table %s: %v", astColDef.Name.Value, s.TableName.Value, err)
				return nil, fmt.Errorf("invalid data type for column %q ('%s'): %w",
					astColDef.Name.Value, astColDef.DataType.String(), err)
			}
			columnSchemas = append(columnSchemas, database.ColumnSchema{
				Name:      astColDef.Name.Value,
				Type:      colType,
				Length:    colParams.Length,
				Precision: colParams.Precision,
				Scale:     colParams.Scale,
			})
		}
		err := db.CreateTable(s.TableName.Value, columnSchemas)
		if err != nil { log.Printf("Error creating table %s: %v", s.TableName.Value, err); return nil, err }
		log.Printf("Successfully created table %s for %s", s.TableName.Value, remoteAddr)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			return writer.Complete("CREATE TABLE")
		}
		return psqlwire.Prepared(psqlwire.NewStatement(stmtFn)), nil

	case *ast.InsertStatement:
		log.Printf("Executing INSERT INTO statement for %s: %s", remoteAddr, s.TableName.Value)
		table, err := db.GetTable(s.TableName.Value); if err != nil { return nil, err }
		var rowsAffectedCount int64 = 0
		for _, astRowValues := range s.Values {
			if len(astRowValues) != len(table.Schema.Columns) {
				return nil, fmt.Errorf("column count mismatch: table %q has %d columns, but %d values were supplied",
					s.TableName.Value, len(table.Schema.Columns), len(astRowValues))
			}
			newGoRow := make(database.Row, len(astRowValues))
			for i, astExpr := range astRowValues {
				colSchema := table.Schema.Columns[i] // Get target column schema
				var val interface{}
				var errLit error

				switch valNode := astExpr.(type) {
				case *ast.LiteralValue:
					switch colSchema.Type { // Coerce based on column type
					case database.BasicTypeInteger:
						val, errLit = strconv.ParseInt(valNode.Value, 10, 64)
					case database.BasicTypeNumeric, database.BasicTypeFloat:
						val, errLit = strconv.ParseFloat(valNode.Value, 64)
					case database.BasicTypeDate:
						val, errLit = time.Parse(DATE_FORMAT_INPUT, valNode.Value)
					case database.BasicTypeText, database.BasicTypeVarchar: // No parsing needed for string target
						val = valNode.Value
					case database.BasicTypeBoolean: // Handle string 'true'/'false' for boolean column
						sVal := strings.ToLower(valNode.Value)
						if sVal == "true" { val = true } else 
						if sVal == "false" { val = false } else 
						{ errLit = fmt.Errorf("invalid boolean string: %s", valNode.Value) }
					default: // Includes BLOB if needed, or other string-like representations
						val = valNode.Value 
					}
					if errLit != nil { return nil, fmt.Errorf("error parsing literal %q for column %q (%s): %w", valNode.Value, colSchema.Name, colSchema.Type, errLit)}
				case *ast.BooleanLiteral: // TRUE or FALSE keyword
					if colSchema.Type != database.BasicTypeBoolean {
						return nil, fmt.Errorf("cannot insert boolean literal %t into non-boolean column %q (%s)", valNode.Value, colSchema.Name, colSchema.Type)
					}
					val = valNode.Value
				default:
					return nil, fmt.Errorf("unsupported value type %T in INSERT; only literals supported", astExpr)
				}
				newGoRow[i] = val
			}
			err := db.InsertRow(s.TableName.Value, newGoRow)
			if err != nil { log.Printf("Error inserting row into table %s: %v", s.TableName.Value, err); return nil, err }
			rowsAffectedCount++
		}
		log.Printf("Successfully inserted %d row(s) into table %s", rowsAffectedCount, s.TableName.Value)
		commandTag := fmt.Sprintf("INSERT 0 %d", rowsAffectedCount)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			return writer.Complete(commandTag)
		}
		return psqlwire.Prepared(psqlwire.NewStatement(stmtFn)), nil

	case *ast.SelectStatement:
		log.Printf("Executing SELECT statement for %s: %s", remoteAddr, s.TableName.Value)
		table, err := db.GetTable(s.TableName.Value); if err != nil { return nil, err }
		rowsToProcess := table.Rows
		if s.WhereClause != nil {
			log.Printf("Applying WHERE clause for SELECT on %s: %s", s.TableName.Value, s.WhereClause.String())
			filteredRows := make([]database.Row, 0)
			for _, row := range table.Rows {
				evalResult, evalErr := evaluator.Eval(s.WhereClause, row, table.Schema)
				if evalErr != nil { log.Printf("Error evaluating WHERE clause: %v. Row excluded.", evalErr); continue }
				if evalResult == nil { continue } // UNKNOWN treated as false
				boolResult, ok := evalResult.(bool)
				if !ok { log.Printf("WHERE clause non-boolean result (%T): %v. Row excluded.", evalResult, evalResult); continue }
				if boolResult { filteredRows = append(filteredRows, row) }
			}
			rowsToProcess = filteredRows
			log.Printf("WHERE clause applied. %d rows remaining for projection.", len(rowsToProcess))
		} else {
			log.Printf("No WHERE clause. Processing all %d rows for projection.", len(rowsToProcess))
		}

		var selectedColumnNames []string
		if len(s.Columns) == 1 { if _, ok := s.Columns[0].(*ast.StarSelectColumn); ok { selectedColumnNames = []string{"*"} } }
		if selectedColumnNames == nil { /* if not SELECT * */
			selectedColumnNames = make([]string, 0, len(s.Columns))
			for _, colExpr := range s.Columns {
				ident, ok := colExpr.(*ast.Identifier) // TODO: Support expressions in SELECT list
				if !ok { return nil, fmt.Errorf("unsupported select list item type: %T (only direct columns or '*' supported for now)", colExpr) }
				selectedColumnNames = append(selectedColumnNames, ident.Value)
			}
		}
		
		resultSchemaForPsql, finalRowsForPsql, err := database.ProjectRows(rowsToProcess, table.Schema, selectedColumnNames)
		if err != nil { log.Printf("Error projecting rows for table %s: %v", s.TableName.Value, err); return nil, err }
		
		psqlwireCols := make(psqlwire.Columns, len(resultSchemaForPsql.Columns))
		for i, colSchema := range resultSchemaForPsql.Columns {
			pgOid := database.BasicTypeToPgOid(colSchema.Type)
			psqlwireCols[i] = psqlwire.Column{ Name: colSchema.Name, Oid: pgOid, Format: psqlwire.TextFormat }
		}

		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			var rowCountSent int64 = 0
			for _, dbRow := range finalRowsForPsql {
				rowDataForWire := make([]interface{}, len(dbRow))
				for i, val := range dbRow {
					if val == nil { rowDataForWire[i] = nil; continue }
					switch v := val.(type) {
					case int64:   rowDataForWire[i] = strconv.FormatInt(v, 10)
					case float64: rowDataForWire[i] = strconv.FormatFloat(v, 'f', -1, 64) // Use schema's Scale for formatting if available
					case string:  rowDataForWire[i] = v
					case []byte:  rowDataForWire[i] = string(v) 
					case bool:    rowDataForWire[i] = strconv.FormatBool(v)
					case time.Time: rowDataForWire[i] = v.Format(DATE_FORMAT_OUTPUT)
					default:      rowDataForWire[i] = fmt.Sprintf("%v", v)
					}
				}
				if err := writer.Row(rowDataForWire); err != nil {
					log.Printf("Error writing data row for SELECT on %s: %v", s.TableName.Value, err); return fmt.Errorf("failed to write data row: %w", err)
				}
				rowCountSent++
			}
			log.Printf("Successfully sent %d row(s) for SELECT on table %s", rowCountSent, s.TableName.Value)
			return writer.Complete(fmt.Sprintf("SELECT %d", rowCountSent))
		}
		return psqlwire.Prepared(psqlwire.NewStatement(stmtFn, psqlwire.WithColumns(psqlwireCols))), nil

	case *ast.SaveStatement:
		log.Printf("Executing SAVE statement for %s: %s", remoteAddr, s.FilePath.Value)
		err := db.SaveToFile(s.FilePath.Value)
		if err != nil { log.Printf("Error saving database: %v", err); return nil, err }
		log.Printf("Successfully saved database to %s for %s", s.FilePath.Value, remoteAddr)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error { return writer.Complete("SAVE 1") }
		return psqlwire.Prepared(psqlwire.NewStatement(stmtFn)), nil

	case *ast.LoadStatement:
		log.Printf("Executing LOAD statement for %s: %s", remoteAddr, s.FilePath.Value)
		err := db.LoadFromFile(s.FilePath.Value)
		if err != nil { log.Printf("Error loading database: %v", err); return nil, err }
		log.Printf("Successfully loaded database from %s for %s", s.FilePath.Value, remoteAddr)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error { return writer.Complete("LOAD 1") }
		return psqlwire.Prepared(psqlwire.NewStatement(stmtFn)), nil

	default:
		log.Printf("Unsupported AST statement type for execution: %T for %s", s, remoteAddr)
		return nil, fmt.Errorf("unsupported statement type for execution: %T", s)
	}
}

```
