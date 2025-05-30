package database

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jeroenrinzema/psql-wire/pkg/oid" // For BasicTypeToPgOid
)

// BasicType represents the simplified, internally recognized data types for columns.
type BasicType int

const (
	BasicTypeUnknown BasicType = iota
	BasicTypeInteger
	BasicTypeText
	BasicTypeFloat
	BasicTypeBlob
)

// String returns a human-readable representation of the BasicType.
func (bt BasicType) String() string {
	switch bt {
	case BasicTypeInteger:
		return "INTEGER"
	case BasicTypeText:
		return "TEXT"
	case BasicTypeFloat:
		return "FLOAT"
	case BasicTypeBlob:
		return "BLOB"
	default:
		return "UNKNOWN"
	}
}

// StringToBasicType converts a type name string to a BasicType.
func StringToBasicType(typeName string) (BasicType, error) {
	upperTypeName := strings.ToUpper(typeName)
	switch upperTypeName {
	case "INTEGER", "INT", "BIGINT", "SMALLINT", "TINYINT":
		return BasicTypeInteger, nil
	case "TEXT", "VARCHAR", "CHAR", "CLOB":
		return BasicTypeText, nil
	case "REAL", "FLOAT", "DOUBLE", "NUMERIC", "DECIMAL":
		return BasicTypeFloat, nil
	case "BLOB":
		return BasicTypeBlob, nil
	default:
		return BasicTypeUnknown, fmt.Errorf("unsupported data type: %q", typeName)
	}
}

// BasicTypeToPgOid maps an internal BasicType to a PostgreSQL OID.
func BasicTypeToPgOid(bt BasicType) oid.Oid {
	switch bt {
	case BasicTypeInteger:
		return oid.Int8 // Using Int8 as SQLite INTEGER can be up to 64-bit
	case BasicTypeText:
		return oid.Text
	case BasicTypeFloat:
		return oid.Float8
	case BasicTypeBlob:
		return oid.Bytea
	default:
		// Fallback for BasicTypeUnknown or any other unmapped types
		return oid.Text // Or oid.Unknown / some other appropriate default
	}
}

// ColumnSchema defines the name and type of a single column.
type ColumnSchema struct {
	Name string
	Type BasicType
}

// TableSchema defines the structure of a table.
type TableSchema struct {
	Name    string
	Columns []ColumnSchema
	ColMap  map[string]ColumnSchema // Map key is lowercased column name
}

// NewTableSchema creates a new table schema.
func NewTableSchema(name string, columns []ColumnSchema) *TableSchema {
	ts := &TableSchema{
		Name:    name, // Preserves original table name casing
		Columns: columns,
		ColMap:  make(map[string]ColumnSchema, len(columns)),
	}
	for _, col := range columns {
		ts.ColMap[strings.ToLower(col.Name)] = col
	}
	return ts
}

// Row represents a single row of data within a table.
type Row []interface{}

// Table represents a single table including its schema and all its data.
type Table struct {
	Schema *TableSchema
	Rows   []Row
}

// NewTable creates a new table with the given schema.
func NewTable(schema *TableSchema) *Table {
	return &Table{
		Schema: schema,
		Rows:   make([]Row, 0),
	}
}

// Database represents the entire in-memory database.
type Database struct {
	Tables map[string]*Table // Map key is lowercased table name
}

// NewDatabase creates a new in-memory database instance.
func NewDatabase() *Database {
	return &Database{
		Tables: make(map[string]*Table),
	}
}

// CreateTable adds a new table to the database.
func (db *Database) CreateTable(name string, columns []ColumnSchema) error {
	lowerName := strings.ToLower(name)
	if _, exists := db.Tables[lowerName]; exists {
		return fmt.Errorf("table %q already exists", name)
	}
	schema := NewTableSchema(name, columns)
	db.Tables[lowerName] = NewTable(schema)
	return nil
}

// InsertRow adds a new row to the specified table.
func (db *Database) InsertRow(tableName string, row Row) error {
	lowerTableName := strings.ToLower(tableName)
	table, exists := db.Tables[lowerTableName]
	if !exists {
		return fmt.Errorf("table %q does not exist", tableName)
	}

	if len(row) != len(table.Schema.Columns) {
		return fmt.Errorf("column count mismatch: table %q has %d columns, but %d values were supplied",
			table.Schema.Name, len(table.Schema.Columns), len(row))
	}

	for i, cellValue := range row {
		colSchema := table.Schema.Columns[i]
		// Allow nil values regardless of column type
		if cellValue == nil {
			// TODO: Add NOT NULL constraint check here if/when implemented
			continue
		}
		switch colSchema.Type {
		case BasicTypeInteger:
			if _, ok_int64 := cellValue.(int64); !ok_int64 {
				if _, ok_int := cellValue.(int); !ok_int {
					return fmt.Errorf("type mismatch for column %q: expected %s, got %T for value '%v'",
						colSchema.Name, colSchema.Type.String(), cellValue, cellValue)
				}
			}
		case BasicTypeText:
			if _, ok := cellValue.(string); !ok {
				return fmt.Errorf("type mismatch for column %q: expected %s, got %T for value '%v'",
					colSchema.Name, colSchema.Type.String(), cellValue, cellValue)
			}
		case BasicTypeFloat:
			if _, ok := cellValue.(float64); !ok {
				return fmt.Errorf("type mismatch for column %q: expected %s, got %T for value '%v'",
					colSchema.Name, colSchema.Type.String(), cellValue, cellValue)
			}
		case BasicTypeBlob:
			if _, ok := cellValue.([]byte); !ok {
				return fmt.Errorf("type mismatch for column %q: expected %s, got %T for value '%v'",
					colSchema.Name, colSchema.Type.String(), cellValue, cellValue)
			}
		}
	}
	table.Rows = append(table.Rows, row)
	return nil
}

// SelectRows retrieves rows from a table based on selected column names.
func (db *Database) SelectRows(tableName string, requestedColumnNames []string) (*TableSchema, []Row, error) {
	lowerTableName := strings.ToLower(tableName)
	table, exists := db.Tables[lowerTableName]
	if !exists {
		return nil, nil, fmt.Errorf("table %q does not exist", tableName)
	}

	var resultSchemaCols []ColumnSchema
	var projectedColIndices []int // Stores indices of columns in the original table.Rows

	if len(requestedColumnNames) == 1 && requestedColumnNames[0] == "*" {
		resultSchemaCols = make([]ColumnSchema, len(table.Schema.Columns))
		copy(resultSchemaCols, table.Schema.Columns) // Use original column schemas

		projectedColIndices = make([]int, len(table.Schema.Columns))
		for i := 0; i < len(table.Schema.Columns); i++ {
			projectedColIndices[i] = i
		}
	} else {
		resultSchemaCols = make([]ColumnSchema, 0, len(requestedColumnNames))
		projectedColIndices = make([]int, 0, len(requestedColumnNames))
		for _, reqName := range requestedColumnNames {
			lowerReqName := strings.ToLower(reqName)
			colSchema, ok := table.Schema.ColMap[lowerReqName]
			if !ok {
				return nil, nil, fmt.Errorf("column %q does not exist in table %q", reqName, table.Schema.Name)
			}
			resultSchemaCols = append(resultSchemaCols, colSchema) // Add the found ColumnSchema

			// Find the original index of this column for row projection
			foundOriginalIndex := -1
			for i, origCol := range table.Schema.Columns {
				if strings.ToLower(origCol.Name) == lowerReqName {
					foundOriginalIndex = i
					break
				}
			}
			// This should always be found if it was in ColMap, but as a safeguard:
			if foundOriginalIndex == -1 {
				return nil, nil, fmt.Errorf("internal error: column %q found in ColMap but not in Columns list", reqName)
			}
			projectedColIndices = append(projectedColIndices, foundOriginalIndex)
		}
	}
	
	// The result schema uses the original table name and the selected columns.
	resultSchema := NewTableSchema(table.Schema.Name, resultSchemaCols)

	projectedRows := make([]Row, 0, len(table.Rows))
	for _, originalRow := range table.Rows {
		newRow := make(Row, len(projectedColIndices))
		for i, originalIndex := range projectedColIndices {
			newRow[i] = originalRow[originalIndex]
		}
		projectedRows = append(projectedRows, newRow)
	}

	return resultSchema, projectedRows, nil
}
```
