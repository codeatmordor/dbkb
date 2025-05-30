package database

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"inmempg/ast" // Required for StringToBasicTypeExtended tests
	"inmempg/token" // Required for StringToBasicTypeExtended tests (for ast.LiteralValue token type)
	psqlwire_oid "github.com/jeroenrinzema/psql-wire/pkg/oid" // Renamed to avoid conflict
)

func TestStringToBasicTypeExtended(t *testing.T) {
	// Helper to create ast.LiteralValue for type params
	intLit := func(s string) *ast.LiteralValue {
		return &ast.LiteralValue{Token: token.Token{Type: token.INT, Literal: s}, Value: s}
	}

	tests := []struct {
		name            string
		typeIdent       *ast.Identifier
		typeParams      []ast.Expression
		expectedType    BasicType
		expectedColParams ColumnParams
		expectError     bool
		errorContains   string
	}{
		{"INTEGER", &ast.Identifier{Value: "INTEGER"}, nil, BasicTypeInteger, ColumnParams{}, false, ""},
		{"TEXT", &ast.Identifier{Value: "TEXT"}, nil, BasicTypeText, ColumnParams{Length: -1}, false, ""},
		{"BOOLEAN", &ast.Identifier{Value: "BOOLEAN"}, nil, BasicTypeBoolean, ColumnParams{}, false, ""},
		{"DATE", &ast.Identifier{Value: "DATE"}, nil, BasicTypeDate, ColumnParams{}, false, ""},
		{"VARCHAR(50)", &ast.Identifier{Value: "VARCHAR"}, []ast.Expression{intLit("50")}, BasicTypeVarchar, ColumnParams{Length: 50}, false, ""},
		{"NUMERIC(10,2)", &ast.Identifier{Value: "NUMERIC"}, []ast.Expression{intLit("10"), intLit("2")}, BasicTypeNumeric, ColumnParams{Precision: 10, Scale: 2}, false, ""},
		{"NUMERIC(8)", &ast.Identifier{Value: "NUMERIC"}, []ast.Expression{intLit("8")}, BasicTypeNumeric, ColumnParams{Precision: 8, Scale: 0}, false, ""}, // Scale defaults to 0
		{"NUMERIC", &ast.Identifier{Value: "NUMERIC"}, nil, BasicTypeNumeric, ColumnParams{Precision:0, Scale:0}, false, ""}, // No params, implementation defined (current: P=0,S=0)

		// Error cases
		{"VARCHAR no len", &ast.Identifier{Value: "VARCHAR"}, nil, BasicTypeUnknown, ColumnParams{}, true, "VARCHAR requires exactly one length parameter"},
		{"VARCHAR non-int len", &ast.Identifier{Value: "VARCHAR"}, []ast.Expression{&ast.LiteralValue{Value: "abc"}}, BasicTypeUnknown, ColumnParams{}, true, "expected integer literal"},
		{"VARCHAR zero len", &ast.Identifier{Value: "VARCHAR"}, []ast.Expression{intLit("0")}, BasicTypeUnknown, ColumnParams{}, true, "length must be positive"},
		{"NUMERIC non-int precision", &ast.Identifier{Value: "NUMERIC"}, []ast.Expression{&ast.LiteralValue{Value: "abc"}}, BasicTypeUnknown, ColumnParams{}, true, "expected integer literal"},
		{"NUMERIC scale > precision", &ast.Identifier{Value: "NUMERIC"}, []ast.Expression{intLit("5"), intLit("10")}, BasicTypeUnknown, ColumnParams{}, true, "scale (10) cannot be greater than precision (5)"},
        {"UNSUPPORTED_TYPE", &ast.Identifier{Value: "XML"}, nil, BasicTypeUnknown, ColumnParams{}, true, "unsupported data type: \"XML\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotParams, err := StringToBasicTypeExtended(tt.typeIdent, tt.typeParams)
			if tt.expectError {
				if err == nil {
					t.Fatalf("StringToBasicTypeExtended(%s, %v) expected error, got nil", tt.typeIdent.Value, tt.typeParams)
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("StringToBasicTypeExtended(%s, %v) unexpected error: %v", tt.typeIdent.Value, tt.typeParams, err)
				}
				if gotType != tt.expectedType {
					t.Errorf("StringToBasicTypeExtended type = %v, want %v", gotType, tt.expectedType)
				}
				if !reflect.DeepEqual(gotParams, tt.expectedColParams) {
					t.Errorf("StringToBasicTypeExtended params = %+v, want %+v", gotParams, tt.expectedColParams)
				}
			}
		})
	}
}


func TestDatabaseCreateTableWithNewTypes(t *testing.T) {
	db := NewDatabase()
	cols := []ColumnSchema{
		{Name: "id", Type: BasicTypeInteger},
		{Name: "description", Type: BasicTypeVarchar, Length: 100},
		{Name: "is_active", Type: BasicTypeBoolean},
		{Name: "created_at", Type: BasicTypeDate},
		{Name: "amount", Type: BasicTypeNumeric, Precision: 12, Scale: 2},
        {Name: "code", Type: BasicTypeText, Length: -1}, // TEXT
	}
	err := db.CreateTable("new_features_table", cols)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	tbl, exists := db.Tables["new_features_table"]
	if !exists {
		t.Fatalf("Table 'new_features_table' not created")
	}

	expectedSchema := map[string]ColumnSchema{
		"id":          {Name: "id", Type: BasicTypeInteger},
		"description": {Name: "description", Type: BasicTypeVarchar, Length: 100},
		"is_active":   {Name: "is_active", Type: BasicTypeBoolean},
		"created_at":  {Name: "created_at", Type: BasicTypeDate},
		"amount":      {Name: "amount", Type: BasicTypeNumeric, Precision: 12, Scale: 2},
        "code":        {Name: "code", Type: BasicTypeText, Length: -1},
	}

	if len(tbl.Schema.Columns) != len(expectedSchema) {
		t.Fatalf("Incorrect number of columns. Got %d, want %d", len(tbl.Schema.Columns), len(expectedSchema))
	}

	for _, actualCol := range tbl.Schema.Columns {
		lowerName := strings.ToLower(actualCol.Name)
		expectedCol, ok := expectedSchema[lowerName]
		if !ok {
			t.Errorf("Unexpected column %s in schema", actualCol.Name)
			continue
		}
		if actualCol.Name != expectedCol.Name || // Check original name casing
			actualCol.Type != expectedCol.Type ||
			actualCol.Length != expectedCol.Length ||
			actualCol.Precision != expectedCol.Precision ||
			actualCol.Scale != expectedCol.Scale {
			t.Errorf("ColumnSchema mismatch for %s. Got %+v, want %+v", actualCol.Name, actualCol, expectedCol)
		}
	}
}

func TestDatabaseInsertRowWithNewTypes(t *testing.T) {
	db := NewDatabase()
	cols := []ColumnSchema{
		{Name: "name", Type: BasicTypeVarchar, Length: 10},
		{Name: "active", Type: BasicTypeBoolean},
		{Name: "event_date", Type: BasicTypeDate},
		{Name: "price", Type: BasicTypeNumeric, Precision: 5, Scale: 2},
	}
	if err := db.CreateTable("type_test_table", cols); err != nil {
		t.Fatalf("Setup CreateTable failed: %v", err)
	}

	now := time.Now().Truncate(24 * time.Hour) // For date comparison

	validRows := []struct {
		name string
		row  Row
	}{
		{"Valid full row", Row{"short", true, now, float64(123.45)}},
		{"Valid int for numeric", Row{"item2", false, now.AddDate(0,0,1), int64(50)}}, // int64 for NUMERIC
	}
	for _, tt := range validRows {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.InsertRow("type_test_table", tt.row); err != nil {
				t.Errorf("InsertRow failed for %s: %v", tt.name, err)
			}
		})
	}
	
	// Clear rows for error tests
	db.Tables["type_test_table"].Rows = make([]Row, 0)

	errorRows := []struct {
		name          string
		row           Row
		errorContains string
	}{
		{"Varchar too long", Row{"verylongstring", false, now, float64(10.00)}, "value too long for column \"name\" (VARCHAR(10))"},
		{"Int for Boolean", Row{"oklen", 123, now, float64(10.00)}, "type mismatch for column \"active\" (BOOLEAN): expected BOOLEAN, got int"},
		{"String for Date (invalid format)", Row{"oklen", true, "not-a-date", float64(10.00)}, "type mismatch for column \"event_date\" (DATE): expected DATE (time.Time), got string"},
		{"String for Numeric", Row{"oklen", true, now, "not-a-number"}, "type mismatch for column \"price\" (NUMERIC): expected NUMERIC compatible, got string"},
	}
	for _, tt := range errorRows {
		t.Run(tt.name, func(t *testing.T) {
			err := db.InsertRow("type_test_table", tt.row)
			if err == nil {
				t.Errorf("Expected error for %s but got nil", tt.name)
			} else if !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("For %s, expected error containing '%s', got '%s'", tt.name, tt.errorContains, err.Error())
			}
		})
	}
}


func TestDatabaseSaveLoad(t *testing.T) {
	db := NewDatabase()
	originalCols := []ColumnSchema{
		{Name: "ID", Type: BasicTypeInteger},
		{Name: "Name", Type: BasicTypeVarchar, Length: 50},
		{Name: "IsActive", Type: BasicTypeBoolean},
		{Name: "BirthDate", Type: BasicTypeDate},
		{Name: "Balance", Type: BasicTypeNumeric, Precision: 10, Scale: 2},
        {Name: "Notes", Type: BasicTypeText, Length: -1},
	}
	if err := db.CreateTable("pers_test", originalCols); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	date1, _ := time.Parse("2006-01-02", "1990-05-15")
	date2, _ := time.Parse("2006-01-02", "1985-11-20")

	rowsToInsert := []Row{
		{int64(1), "Alice Smith", true, date1, float64(1234.56), "First note"},
		{int64(2), "Bob Johnson", false, date2, float64(789.00), "Second note with more text"},
        {int64(3), "Charlie Brown", true, nil, nil, nil}, // Test with nils
	}
	for _, r := range rowsToInsert {
		if err := db.InsertRow("pers_test", r); err != nil {
			t.Fatalf("InsertRow failed: %v", err)
		}
	}

	// Save to a temporary file
	tempDir := t.TempDir() // Creates a temporary directory that is cleaned up after the test
	filePath := filepath.Join(tempDir, "test_db.gob")

	err := db.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Create a new DB instance and load
	db2 := NewDatabase()
	err = db2.LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify loaded data
	if len(db2.Tables) != 1 {
		t.Fatalf("Loaded DB should have 1 table, got %d", len(db2.Tables))
	}
	loadedTable, exists := db2.Tables["pers_test"] // Map key is lowercase
	if !exists {
		t.Fatalf("Table 'pers_test' not found in loaded DB")
	}

	// Verify Schema (deep equal might be too strict if unexported fields differ, but good for exported)
	if !reflect.DeepEqual(loadedTable.Schema, db.Tables["pers_test"].Schema) {
		t.Errorf("Loaded table schema mismatch.\nGot: %+v\nWant: %+v", loadedTable.Schema, db.Tables["pers_test"].Schema)
		// More granular checks if DeepEqual fails due to map order or internal things:
		if loadedTable.Schema.Name != db.Tables["pers_test"].Schema.Name {
			t.Errorf("Schema name mismatch: got %s, want %s", loadedTable.Schema.Name, db.Tables["pers_test"].Schema.Name)
		}
		if len(loadedTable.Schema.Columns) != len(db.Tables["pers_test"].Schema.Columns) {
			t.Errorf("Schema column count mismatch: got %d, want %d", len(loadedTable.Schema.Columns), len(db.Tables["pers_test"].Schema.Columns))
		} else {
			for i := range loadedTable.Schema.Columns {
				if !reflect.DeepEqual(loadedTable.Schema.Columns[i], db.Tables["pers_test"].Schema.Columns[i]) {
					t.Errorf("Schema column %d mismatch: \nGot: %+v\nWant: %+v", i, loadedTable.Schema.Columns[i], db.Tables["pers_test"].Schema.Columns[i])
				}
			}
		}
	}


	// Verify Rows
	if len(loadedTable.Rows) != len(rowsToInsert) {
		t.Fatalf("Loaded table should have %d rows, got %d", len(rowsToInsert), len(loadedTable.Rows))
	}
	for i, expectedRow := range rowsToInsert {
		if !reflect.DeepEqual(loadedTable.Rows[i], expectedRow) {
			t.Errorf("Row %d data mismatch.\nGot:  %v (%T)\nWant: %v (%T)", i, loadedTable.Rows[i], loadedTable.Rows[i], expectedRow, expectedRow)
			// Check individual elements if DeepEqual fails
			for j, cell := range loadedTable.Rows[i] {
				if !reflect.DeepEqual(cell, expectedRow[j]) {
					t.Errorf("Row %d, Cell %d mismatch: Got %v (%T), Want %v (%T)", i, j, cell, cell, expectedRow[j], expectedRow[j])
				}
			}
		}
	}

	// Test loading non-existent file
	err = db2.LoadFromFile("non_existent_file.gob")
	if err == nil {
		t.Errorf("Expected error when loading non-existent file, got nil")
	}
}

// TestDatabaseSelectRows - existing tests should be fine, but ensure they cover new types if necessary
// For this task, the focus was more on Create/Insert/Save/Load with new types.
// SelectRows' projection logic is type-agnostic as it deals with []interface{}.
// The important part for SELECT is that data is inserted correctly and can be retrieved.
// BasicTypeToPgOid has its own test.

// BasicTypeToPgOid test (from previous step, ensure it's here or combined)
func TestBasicTypeToPgOid(t *testing.T) {
	tests := []struct {
		input    BasicType
		expected psqlwire_oid.Oid
	}{
		{BasicTypeInteger, psqlwire_oid.Int8},
		{BasicTypeText, psqlwire_oid.Text},
		{BasicTypeVarchar, psqlwire_oid.Varchar},
		{BasicTypeFloat, psqlwire_oid.Float8},
		{BasicTypeNumeric, psqlwire_oid.Numeric},
		{BasicTypeBlob, psqlwire_oid.Bytea},
		{BasicTypeBoolean, psqlwire_oid.Bool},
		{BasicTypeDate, psqlwire_oid.Date},
		{BasicTypeUnknown, psqlwire_oid.Text}, // Fallback
	}
	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			got := BasicTypeToPgOid(tt.input)
			if got != tt.expected {
				t.Errorf("BasicTypeToPgOid(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

```
