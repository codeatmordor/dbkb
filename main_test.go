package main

import (
	"testing"

	"github.com/jeroenrinzema/psql-wire/pkg/oid"
)

func TestMapSQLiteTypeToPgOid(t *testing.T) {
	testCases := []struct {
		name       string
		sqliteType string
		expectedOid oid.Oid
	}{
		{"TEXT type", "TEXT", oid.Text},
		{"TEXT type lowercase", "text", oid.Text},
		{"VARCHAR type", "VARCHAR", oid.Text},
		{"CHAR type", "CHAR", oid.Text},
		{"CLOB type", "CLOB", oid.Text},
		{"INTEGER type", "INTEGER", oid.Int8},
		{"INTEGER type lowercase", "integer", oid.Int8},
		{"INT type", "INT", oid.Int8},
		{"BIGINT type", "BIGINT", oid.Int8},
		{"MEDIUMINT type", "MEDIUMINT", oid.Int8},
		{"SMALLINT type", "SMALLINT", oid.Int8},
		{"TINYINT type", "TINYINT", oid.Int8},
		{"REAL type", "REAL", oid.Float8},
		{"REAL type lowercase", "real", oid.Float8},
		{"FLOAT type", "FLOAT", oid.Float8},
		{"DOUBLE type", "DOUBLE", oid.Float8},
		{"NUMERIC type", "NUMERIC", oid.Float8}, // Or oid.Numeric if psql-wire supports it well & it's preferred
		{"DECIMAL type", "DECIMAL", oid.Float8}, // Or oid.Numeric
		{"BLOB type", "BLOB", oid.Bytea},
		{"BLOB type lowercase", "blob", oid.Bytea},
		{"DATETIME type", "DATETIME", oid.Timestamp},
		{"TIMESTAMP type", "TIMESTAMP", oid.Timestamp},
		{"DATE type", "DATE", oid.Timestamp}, // Simplified to Timestamp; could be oid.Date
		{"TIME type", "TIME", oid.Timestamp}, // Simplified to Timestamp; could be oid.Time
		{"NULL type", "NULL", oid.Text},      // As per current implementation
		{"Unmapped type", "GEOMETRY", oid.Text}, // Test fallback for unmapped
		{"Empty type", "", oid.Text},           // Test fallback for empty string
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOid := mapSQLiteTypeToPgOid(tc.sqliteType)
			if actualOid != tc.expectedOid {
				t.Errorf("mapSQLiteTypeToPgOid(%q) = %v; want %v", tc.sqliteType, actualOid, tc.expectedOid)
			}
		})
	}
}
