package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/jeroenrinzema/psql-wire/pkg/codes"
	"github.com/jeroenrinzema/psql-wire/pkg/messages"
	"github.com/jeroenrinzema/psql-wire/pkg/oid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jeroenrinzema/psql-wire"
	"github.com/jeroenrinzema/psql-wire/pkg/buffer" // Required for sendErrorResponse if used
)

// Global variable for the SQLite database instance
var sqliteDB *sql.DB

func main() {
	var err error
	sqliteDB, err = initSQLite()
	if err != nil {
		log.Fatalf("Failed to initialize SQLite database: %v", err)
	}

	authStrategy := psqlwire.ClearTextPassword(func(ctx context.Context, database, username, password string) (context.Context, bool, error) {
		log.Printf("Login attempt: db=%s, user=%s, password=[REDACTED]", database, username)
		ctx = context.WithValue(ctx, "database", database)
		ctx = context.WithValue(ctx, "username", username)
		return ctx, true, nil
	})

	opts := []psqlwire.OptionFn{
		psqlwire.SessionAuthStrategy(authStrategy),
	}

	server, err := psqlwire.NewServer(handleQueryParseFn, opts...)
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

// mapSQLiteTypeToPgOid maps SQLite type names to PostgreSQL OIDs.
func mapSQLiteTypeToPgOid(sqliteType string) oid.Oid {
	// SQLite types are often dynamic. DatabaseTypeName() can return various things.
	// Common ones: "TEXT", "INTEGER", "REAL", "BLOB", "NULL".
	// Also, affinity based types like "NUMERIC", "DATETIME" might appear.
	// This mapping is basic and might need refinement.
	upperSqliteType := strings.ToUpper(sqliteType)
	switch upperSqliteType {
	case "TEXT", "VARCHAR", "CHAR", "CLOB":
		return oid.Text
	case "INTEGER", "INT", "BIGINT", "MEDIUMINT", "SMALLINT", "TINYINT":
		return oid.Int8 // Using Int8 for safety as SQLite INTEGER can be 64-bit.
	case "REAL", "FLOAT", "DOUBLE", "NUMERIC", "DECIMAL": // NUMERIC/DECIMAL might be better as oid.Numeric
		return oid.Float8
	case "BLOB":
		return oid.Bytea
	case "NULL": // A column might be typed as NULL if all values in a sample are NULL.
		return oid.Text // Default to Text for NULL type columns
	case "DATETIME", "TIMESTAMP", "DATE", "TIME":
		return oid.Timestamp // Or oid.Date, oid.Time, oid.Timestamptz depending on specifics
	default:
		log.Printf("Unmapped SQLite type: %s, defaulting to oid.Text", sqliteType)
		return oid.Text
	}
}

func handleQueryParseFn(ctx context.Context, query string) (psqlwire.PreparedStatements, error) {
	remoteAddr := psqlwire.RemoteAddress(ctx)
	log.Printf("Received query from %s: %s", remoteAddr, query)

	normalizedQuery := strings.TrimSpace(query)
	lowerQuery := strings.ToLower(normalizedQuery)

	if lowerQuery == "select 1" {
		cols := psqlwire.Columns{
			psqlwire.Column{Name: "?column?", Oid: oid.Int4, Format: psqlwire.TextFormat},
		}
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			log.Printf("Executing 'SELECT 1' for %s", psqlwire.RemoteAddress(execCtx))
			if err := writer.Row([]any{int32(1)}); err != nil {
				return fmt.Errorf("failed to write data row for SELECT 1: %w", err)
			}
			return writer.Complete("SELECT 1")
		}
		preparedStatement := psqlwire.NewStatement(stmtFn, psqlwire.WithColumns(cols))
		return psqlwire.Prepared(preparedStatement), nil
	}

	if strings.HasPrefix(lowerQuery, "create table") {
		// ... (CREATE TABLE handling as before) ...
		log.Printf("Attempting to execute CREATE TABLE statement for %s: %s", remoteAddr, normalizedQuery)
		_, err := sqliteDB.ExecContext(ctx, normalizedQuery)
		if err != nil {
			log.Printf("Error executing CREATE TABLE for %s: %v. Query: %s", remoteAddr, err, normalizedQuery)
			return nil, fmt.Errorf("failed to execute CREATE TABLE: %w", err)
		}
		log.Printf("Successfully executed CREATE TABLE for %s: %s", remoteAddr, normalizedQuery)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			return writer.Complete("CREATE TABLE")
		}
		preparedStatement := psqlwire.NewStatement(stmtFn)
		return psqlwire.Prepared(preparedStatement), nil
	}

	if strings.HasPrefix(lowerQuery, "insert into") {
		// ... (INSERT INTO handling as before) ...
		log.Printf("Attempting to execute INSERT INTO statement for %s: %s", remoteAddr, normalizedQuery)
		result, err := sqliteDB.ExecContext(ctx, normalizedQuery)
		if err != nil {
			log.Printf("Error executing INSERT INTO for %s: %v. Query: %s", remoteAddr, err, normalizedQuery)
			return nil, fmt.Errorf("failed to execute INSERT INTO: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Error getting rows affected for INSERT INTO for %s: %v. Query: %s", remoteAddr, err, normalizedQuery)
			rowsAffected = 0
		}
		log.Printf("Successfully executed INSERT INTO for %s: %s (%d rows affected)", remoteAddr, normalizedQuery, rowsAffected)
		commandTag := fmt.Sprintf("INSERT 0 %d", rowsAffected)
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			return writer.Complete(commandTag)
		}
		preparedStatement := psqlwire.NewStatement(stmtFn)
		return psqlwire.Prepared(preparedStatement), nil
	}

	// Basic check for SELECT ... FROM ...
	if strings.HasPrefix(lowerQuery, "select ") && strings.Contains(lowerQuery, " from ") {
		log.Printf("Attempting to execute SELECT statement for %s: %s", remoteAddr, normalizedQuery)
		
		rows, err := sqliteDB.QueryContext(ctx, normalizedQuery)
		if err != nil {
			log.Printf("Error executing SELECT query for %s: %v. Query: %s", remoteAddr, err, normalizedQuery)
			return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
		}
		// Note: rows.Close() is called in the PreparedStatementFn

		// Get column information for RowDescription
		sqlColumnNames, err := rows.Columns()
		if err != nil {
			rows.Close()
			log.Printf("Error getting column names for %s: %v", remoteAddr, err)
			return nil, fmt.Errorf("failed to get column names: %w", err)
		}

		sqlColumnTypes, err := rows.ColumnTypes()
		if err != nil {
			rows.Close()
			log.Printf("Error getting column types for %s: %v", remoteAddr, err)
			return nil, fmt.Errorf("failed to get column types: %w", err)
		}

		psqlwireCols := make(psqlwire.Columns, len(sqlColumnNames))
		for i, colName := range sqlColumnNames {
			sqliteType := sqlColumnTypes[i].DatabaseTypeName()
			pgOid := mapSQLiteTypeToPgOid(sqliteType)
			psqlwireCols[i] = psqlwire.Column{
				Name:   colName,
				Oid:    pgOid,
				Format: psqlwire.TextFormat, // Using text format for simplicity
			}
			log.Printf("Mapping column %s (SQLite type %s) to OID %d", colName, sqliteType, pgOid)
		}

		// PreparedStatementFn to send rows
		stmtFn := func(execCtx context.Context, writer psqlwire.DataWriter, params []psqlwire.Parameter) error {
			defer rows.Close() // Ensure rows is closed

			var rowCount int64 = 0
			scanArgs := make([]interface{}, len(psqlwireCols))
			rawData := make([][]byte, len(psqlwireCols)) // For converting to []byte before string for text format
			rowData := make([]interface{}, len(psqlwireCols))

			for i := range scanArgs {
				scanArgs[i] = &rawData[i]
			}

			for rows.Next() {
				if err := rows.Scan(scanArgs...); err != nil {
					log.Printf("Error scanning row for %s: %v", psqlwire.RemoteAddress(execCtx), err)
					return fmt.Errorf("failed to scan row: %w", err)
				}

				for i, raw := range rawData {
					if raw == nil {
						rowData[i] = nil // Keep nil as nil for NULL values
					} else {
						// For text format, convert all to string.
						// This is a simplification. Binary format would require more careful type handling.
						// Also, psql-wire might do some conversions based on OID for text, but string is safest.
						valStr := string(raw)
						
						// Attempt to cast to appropriate type based on OID for more accurate representation if possible
						// This is basic, more robust type handling would be needed for production
														switch psqlwireCols[i].Oid {
														case oid.Int2, oid.Int4, oid.Int8:
																// Try to parse as int if possible, otherwise send as string
																// For simplicity, keeping as string as TextFormat is used.
																// If binary format were used, actual int types would be needed.
																rowData[i] = valStr
														case oid.Float4, oid.Float8:
																rowData[i] = valStr
														default:
																rowData[i] = valStr
														}
					}
				}
				
				if err := writer.Row(rowData); err != nil {
					log.Printf("Error writing data row for %s: %v", psqlwire.RemoteAddress(execCtx), err)
					return fmt.Errorf("failed to write data row: %w", err)
				}
				rowCount++
			}

			if err := rows.Err(); err != nil {
				log.Printf("Error iterating rows for %s: %v", psqlwire.RemoteAddress(execCtx), err)
				return fmt.Errorf("row iteration error: %w", err)
			}
			
			log.Printf("Successfully sent %d rows for SELECT query from %s", rowCount, psqlwire.RemoteAddress(execCtx))
			return writer.Complete(fmt.Sprintf("SELECT %d", rowCount))
		}

		preparedStatement := psqlwire.NewStatement(stmtFn, psqlwire.WithColumns(psqlwireCols))
		return psqlwire.Prepared(preparedStatement), nil
	}

	errMsg := fmt.Sprintf("Unsupported query: \"%s\". Only 'SELECT 1', 'CREATE TABLE ...', 'INSERT INTO ...', and 'SELECT * FROM ...' are supported.", query)
	log.Printf("Unsupported query from %s: %s", remoteAddr, query)
	return nil, fmt.Errorf(errMsg)
}

func initSQLite() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}
	log.Println("In-memory SQLite database initialized.")
	return db, nil
}

func sendErrorResponse(ctx context.Context, conn net.Conn, code, errMsg string) {
	writer := buffer.NewWriter(conn)
	err := messages.Error{
		Severity: messages.ErrorSeverityError,
		Code:     codes.Code(code),
		Message:  errMsg,
	}.Encode(writer)
	if err != nil {
		log.Printf("Failed to encode error message: %v", err)
		return
	}
	if err := writer.WriteTo(conn); err != nil {
		log.Printf("Failed to send error response to %s: %v", conn.RemoteAddr(), err)
	}
}
```
