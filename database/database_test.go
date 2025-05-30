package database

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jeroenrinzema/psql-wire/pkg/oid" // For BasicTypeToPgOid tests
)

func TestStringToBasicType(t *testing.T) {
	tests := []struct {
		input    string
		expected BasicType
		hasError bool
	}{
		{"INTEGER", BasicTypeInteger, false},
		{"int", BasicTypeInteger, false},
		{"TEXT", BasicTypeText, false},
		{"varchar", BasicTypeText, false},
		{"REAL", BasicTypeFloat, false},
		{"float", BasicTypeFloat, false},
		{"BLOB", BasicTypeBlob, false},
		{"BOOLEAN", BasicTypeUnknown, true}, // Example of unsupported
		{"", BasicTypeUnknown, true},        // Empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := StringToBasicType(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("StringToBasicType(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("StringToBasicType(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.expected {
					t.Errorf("StringToBasicType(%q) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestBasicTypeToPgOid(t *testing.T) {
	tests := []struct {
		input    BasicType
		expected oid.Oid
	}{
		{BasicTypeInteger, oid.Int8},
		{BasicTypeText, oid.Text},
		{BasicTypeFloat, oid.Float8},
		{BasicTypeBlob, oid.Bytea},
		{BasicTypeUnknown, oid.Text}, // Fallback
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


func TestDatabaseCreateTable(t *testing.T) {
	db := NewDatabase()

	cols1 := []ColumnSchema{
		{Name: "id", Type: BasicTypeInteger},
		{Name: "name", Type: BasicTypeText},
	}
	err := db.CreateTable("users", cols1)
	if err != nil {
		t.Fatalf("CreateTable('users', ...) failed: %v", err)
	}

	if _, exists := db.Tables["users"]; !exists {
		t.Fatalf("Table 'users' not found in db.Tables after creation")
	}
	if db.Tables["users"].Schema.Name != "users" {
		t.Errorf("Table name in schema is incorrect. got=%s, want='users'", db.Tables["users"].Schema.Name)
	}
	if len(db.Tables["users"].Schema.Columns) != 2 {
		t.Errorf("Incorrect number of columns. got=%d, want=2", len(db.Tables["users"].Schema.Columns))
	}
	if db.Tables["users"].Schema.Columns[0].Name != "id" || db.Tables["users"].Schema.Columns[0].Type != BasicTypeInteger {
		t.Errorf("Column 0 schema incorrect. got=%+v, want={Name:id, Type:BasicTypeInteger}", db.Tables["users"].Schema.Columns[0])
	}
	if strings.ToLower(db.Tables["users"].Schema.Columns[1].Name) != "name" || db.Tables["users"].Schema.Columns[1].Type != BasicTypeText {
		t.Errorf("Column 1 schema incorrect. got=%+v, want={Name:name, Type:BasicTypeText}", db.Tables["users"].Schema.Columns[1])
	}

	// Test creating a table that already exists (case-insensitive check for map key)
	err = db.CreateTable("USERS", cols1)
	if err == nil {
		t.Errorf("Expected error when creating table 'USERS' (already exists as 'users'), but got nil")
	} else {
		expectedErr := "table \"USERS\" already exists"
		if !strings.Contains(err.Error(), "already exists") { // Error message might vary slightly
			t.Errorf("Expected error message containing 'already exists', got %q", err.Error())
		}
	}

	// Test creating table with different casing for columns
	cols2 := []ColumnSchema{
		{Name: "ProductID", Type: BasicTypeInteger},
	}
	err = db.CreateTable("Products", cols2)
	if err != nil {
		t.Fatalf("CreateTable('Products', ...) failed: %v", err)
	}
	prodTable, ok := db.Tables["products"]
	if !ok {
		t.Fatal("Table 'products' (key) not found")
	}
	if prodTable.Schema.Columns[0].Name != "ProductID" { // Check original casing preserved in schema
		t.Errorf("Column name casing not preserved. got=%s, want='ProductID'", prodTable.Schema.Columns[0].Name)
	}
	if _, ok := prodTable.Schema.ColMap["productid"]; !ok { // Check ColMap key is lowercase
		t.Error("ColMap key 'productid' not found")
	}


}

func TestDatabaseInsertRow(t *testing.T) {
	db := NewDatabase()
	userCols := []ColumnSchema{
		{Name: "id", Type: BasicTypeInteger},
		{Name: "name", Type: BasicTypeText},
		{Name: "age", Type: BasicTypeInteger},
	}
	err := db.CreateTable("users", userCols)
	if err != nil {
		t.Fatalf("Setup: CreateTable('users', ...) failed: %v", err)
	}

	// Successful insert
	row1 := Row{int64(1), "Alice", int64(30)}
	err = db.InsertRow("users", row1)
	if err != nil {
		t.Errorf("InsertRow('users', valid_row) failed: %v", err)
	}
	if len(db.Tables["users"].Rows) != 1 {
		t.Fatalf("Expected 1 row after insert, got %d", len(db.Tables["users"].Rows))
	}
	if !reflect.DeepEqual(db.Tables["users"].Rows[0], row1) {
		t.Errorf("Inserted row content mismatch. got=%v, want=%v", db.Tables["users"].Rows[0], row1)
	}

	// Successful insert with int for an int64 column
	rowInt := Row{int(2), "Bob", int(25)}
	err = db.InsertRow("users", rowInt)
	if err != nil {
		t.Errorf("InsertRow('users', row with int) failed: %v", err)
	}
	if len(db.Tables["users"].Rows) != 2 {
		t.Fatalf("Expected 2 rows after second insert, got %d", len(db.Tables["users"].Rows))
	}
	// Verify that int(2) was stored (it will be int type, not int64, but InsertRow allows it)
	retrievedRow := db.Tables["users"].Rows[1]
	if val, ok := retrievedRow[0].(int); !ok || val != 2 {
		t.Errorf("Expected int(2) for id, got %T %v", retrievedRow[0], retrievedRow[0])
	}


	// Insert into non-existent table
	err = db.InsertRow("products", Row{int64(100)})
	if err == nil {
		t.Error("Expected error when inserting into non-existent table, but got nil")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Error message for non-existent table incorrect: %q", err.Error())
	}

	// Column count mismatch (too few)
	err = db.InsertRow("users", Row{int64(2)})
	if err == nil {
		t.Error("Expected error for column count mismatch (too few), but got nil")
	} else if !strings.Contains(err.Error(), "column count mismatch") {
		t.Errorf("Error message for column count mismatch (too few) incorrect: %q", err.Error())
	}

	// Column count mismatch (too many)
	err = db.InsertRow("users", Row{int64(3), "Charlie", int64(40), "extra"})
	if err == nil {
		t.Error("Expected error for column count mismatch (too many), but got nil")
	} else if !strings.Contains(err.Error(), "column count mismatch") {
		t.Errorf("Error message for column count mismatch (too many) incorrect: %q", err.Error())
	}
	
	// Type mismatch (string for INTEGER)
	err = db.InsertRow("users", Row{"wrong", "David", int64(50)})
	if err == nil {
		t.Error("Expected error for type mismatch (string for INTEGER), but got nil")
	} else if !strings.Contains(err.Error(), "type mismatch for column \"id\"") {
		t.Errorf("Error message for type mismatch (string for INTEGER) incorrect: %q", err.Error())
	}

	// Type mismatch (int64 for TEXT)
	err = db.InsertRow("users", Row{int64(4), int64(123), int64(60)})
	if err == nil {
		t.Error("Expected error for type mismatch (int64 for TEXT), but got nil")
	} else if !strings.Contains(err.Error(), "type mismatch for column \"name\"") {
		t.Errorf("Error message for type mismatch (int64 for TEXT) incorrect: %q", err.Error())
	}

	// Insert nil value
	rowNil := Row{int64(5), nil, int64(35)}
	err = db.InsertRow("users", rowNil)
	if err != nil {
		t.Errorf("InsertRow('users', row with nil value) failed: %v", err)
	}
	if len(db.Tables["users"].Rows) != 3 { // 2 previous good ones + 1 nil
		t.Fatalf("Expected 3 rows after nil insert, got %d", len(db.Tables["users"].Rows))
	}
	if db.Tables["users"].Rows[2][1] != nil {
		t.Errorf("Expected nil value for name in third row, got %v", db.Tables["users"].Rows[2][1])
	}
}

func TestDatabaseSelectRows(t *testing.T) {
	db := NewDatabase()
	cols := []ColumnSchema{
		{Name: "ID", Type: BasicTypeInteger},
		{Name: "Name", Type: BasicTypeText},
		{Name: "Value", Type: BasicTypeFloat}, // Add a float for variety
	}
	err := db.CreateTable("items", cols)
	if err != nil {
		t.Fatalf("Setup: CreateTable('items', ...) failed: %v", err)
	}

	rowsToInsert := []Row{
		{int64(1), "ItemA", float64(10.99)},
		{int64(2), "ItemB", float64(25.50)},
		{int64(3), "ItemC", float64(5.75)},
	}
	for _, r := range rowsToInsert {
		if err := db.InsertRow("items", r); err != nil {
			t.Fatalf("Setup: InsertRow('items', ...) failed: %v", err)
		}
	}

	// Test SELECT *
	schemaStar, rowsStar, errStar := db.SelectRows("items", []string{"*"})
	if errStar != nil {
		t.Fatalf("SelectRows('items', ['*']) failed: %v", errStar)
	}
	if schemaStar.Name != "items" {
		t.Errorf("SELECT *: Schema name incorrect. got=%s, want='items'", schemaStar.Name)
	}
	if len(schemaStar.Columns) != 3 {
		t.Fatalf("SELECT *: Expected 3 columns in schema, got %d", len(schemaStar.Columns))
	}
	// Check original casing and order
	if schemaStar.Columns[0].Name != "ID" || schemaStar.Columns[1].Name != "Name" || schemaStar.Columns[2].Name != "Value" {
		t.Errorf("SELECT *: Column names/order incorrect in schema. Got names: %s, %s, %s",
			schemaStar.Columns[0].Name, schemaStar.Columns[1].Name, schemaStar.Columns[2].Name)
	}

	if len(rowsStar) != 3 {
		t.Fatalf("SELECT *: Expected 3 rows, got %d", len(rowsStar))
	}
	if !reflect.DeepEqual(rowsStar[0], rowsToInsert[0]) {
		t.Errorf("SELECT *: Row 0 data mismatch. got=%v, want=%v", rowsStar[0], rowsToInsert[0])
	}

	// Test SELECT specific_cols (Name, ID) - note the order
	schemaSpecific, rowsSpecific, errSpecific := db.SelectRows("items", []string{"Name", "ID"})
	if errSpecific != nil {
		t.Fatalf("SelectRows('items', ['Name', 'ID']) failed: %v", errSpecific)
	}
	if schemaSpecific.Name != "items" {
		t.Errorf("SELECT Name,ID: Schema name incorrect. got=%s, want='items'", schemaSpecific.Name)
	}
	if len(schemaSpecific.Columns) != 2 {
		t.Fatalf("SELECT Name,ID: Expected 2 columns in schema, got %d", len(schemaSpecific.Columns))
	}
	if schemaSpecific.Columns[0].Name != "Name" || schemaSpecific.Columns[0].Type != BasicTypeText {
		t.Errorf("SELECT Name,ID: Column 0 schema incorrect. got=%+v", schemaSpecific.Columns[0])
	}
	if schemaSpecific.Columns[1].Name != "ID" || schemaSpecific.Columns[1].Type != BasicTypeInteger {
		t.Errorf("SELECT Name,ID: Column 1 schema incorrect. got=%+v", schemaSpecific.Columns[1])
	}

	if len(rowsSpecific) != 3 {
		t.Fatalf("SELECT Name,ID: Expected 3 rows, got %d", len(rowsSpecific))
	}
	// Verify projected row data and order
	expectedRow0Specific := Row{"ItemA", int64(1)}
	if !reflect.DeepEqual(rowsSpecific[0], expectedRow0Specific) {
		t.Errorf("SELECT Name,ID: Row 0 data mismatch. got=%v, want=%v", rowsSpecific[0], expectedRow0Specific)
	}
	expectedRow1Specific := Row{"ItemB", int64(2)}
    if !reflect.DeepEqual(rowsSpecific[1], expectedRow1Specific) {
		t.Errorf("SELECT Name,ID: Row 1 data mismatch. got=%v, want=%v", rowsSpecific[1], expectedRow1Specific)
	}


	// Test SELECT specific_cols with case variation for column name
	_, _, errCase := db.SelectRows("items", []string{"name"}) // Request "name", stored as "Name"
	if errCase != nil {
		t.Fatalf("SelectRows('items', ['name']) failed (should be case-insensitive for lookup): %v", errCase)
	}


	// Select from non-existent table
	_, _, errNotExist := db.SelectRows("non_existent", []string{"*"})
	if errNotExist == nil {
		t.Error("Expected error selecting from non-existent table, got nil")
	} else if !strings.Contains(errNotExist.Error(), "does not exist") {
		t.Errorf("Error message for non-existent table incorrect: %q", errNotExist.Error())
	}

	// Select non-existent column
	_, _, errNoCol := db.SelectRows("items", []string{"NonExistentColumn"})
	if errNoCol == nil {
		t.Error("Expected error selecting non-existent column, got nil")
	} else if !strings.Contains(errNoCol.Error(), "column \"NonExistentColumn\" does not exist") {
		t.Errorf("Error message for non-existent column incorrect: %q", errNoCol.Error())
	}
}
```
