package evaluator

import (
	"fmt"
	"testing"
	"time"

	"inmempg/ast"
	"inmempg/database"
	"inmempg/lexer" // For parsing expressions in tests
	"inmempg/parser"// For parsing expressions in tests
	"inmempg/token"
)

func testEval(input string, row database.Row, schema *database.TableSchema) (interface{}, error) {
	l := lexer.New(input)
	p := parser.New(l)
	// Assuming the input is a single expression, not a full statement.
	// The parser's ParseStatement() won't work directly.
	// We need to parse an expression. Let's use ParseExpression method.
	// For simplicity in this test setup, we'll make the test input *part* of a dummy statement
	// like "SELECT <input_expr> FROM dummy;" if our parser needs a statement context,
	// or directly call p.parseExpression if it's suitable.
	// The current parser `parseExpression` is called internally.
	// Let's make a helper that parses just one expression.
	// This requires a small modification or addition to the parser if not already present.
	// For now, we'll assume we can get an ast.Expression node for the input string.
	// A simple way: parse "SELECT <input> FROM dummy_table" and extract the first column expression.

	// Simplified parsing for test expressions:
	// We are testing the evaluator, so the AST node must be correct.
	// A full parse of a dummy statement is the most robust way if parseExpression isn't public.
	// Let's assume we construct AST nodes manually for directness in evaluator tests,
	// or use a helper that can parse just an expression string.

	// Let's use a helper `parseExpressionString` which we'll assume exists or can be added.
	// For this test file, we'll manually construct AST nodes for simplicity and focused testing of Eval.
	// If we were testing parser + evaluator integration, we'd parse.
	
	// This test setup will manually create AST nodes.
	// More complex expressions will require more elaborate manual AST construction
	// or a test helper that parses an expression string into an AST node.
	// For now, the tests will focus on nodes Eval is designed to handle.
	
	// The provided Eval function takes ast.Node.
	// The tests below will construct these nodes.
	// No, the prompt implies testing the Eval function directly.
	// We will construct the ast.Node manually for each test case.
	
	// This function is not used in the tests below, as nodes are built manually.
	// If used, it would need a robust way to parse an expression string.
	panic("testEval helper not fully implemented for direct expression string parsing")
}


func TestEvalLiterals(t *testing.T) {
	tests := []struct {
		name         string
		node         ast.Node
		expectedVal  interface{}
		expectedErr  string
	}{
		{"Integer Literal", &ast.LiteralValue{Token: token.Token{Type: token.INT, Literal: "123"}, Value: "123"}, int64(123), ""},
		{"String Literal", &ast.LiteralValue{Token: token.Token{Type: token.STRING, Literal: "hello"}, Value: "hello"}, "hello", ""},
		{"Boolean TRUE", &ast.BooleanLiteral{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, true, ""},
		{"Boolean FALSE", &ast.BooleanLiteral{Token: token.Token{Type: token.FALSE, Literal: "FALSE"}, Value: false}, false, ""},
		{"Invalid Integer", &ast.LiteralValue{Token: token.Token{Type: token.INT, Literal: "abc"}, Value: "abc"}, nil, "could not parse integer literal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := Eval(tt.node, nil, nil)
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error '%s', got nil", tt.expectedErr)
				}
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("Eval() returned unexpected error: %v", err)
			}
			if val != tt.expectedVal {
				t.Errorf("Eval() got = %v (%T), want = %v (%T)", val, val, tt.expectedVal, tt.expectedVal)
			}
		})
	}
}

func TestEvalIdentifier(t *testing.T) {
	schema := database.NewTableSchema("test", []database.ColumnSchema{
		{Name: "id", Type: database.BasicTypeInteger},
		{Name: "name", Type: database.BasicTypeText},
		{Name: "active", Type: database.BasicTypeBoolean},
	})
	row := database.Row{int64(1), "alice", true}

	tests := []struct {
		name        string
		ident       *ast.Identifier
		expectedVal interface{}
		expectedErr string
	}{
		{"Existing Int Column", &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "id"}, Value: "id"}, int64(1), ""},
		{"Existing Str Column", &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "name"}, Value: "name"}, "alice", ""},
		{"Existing Bool Column", &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "active"}, Value: "active"}, true, ""},
		{"Case Insensitive Column", &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "NAME"}, Value: "NAME"}, "alice", ""},
		{"Non-existent Column", &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "salary"}, Value: "salary"}, nil, "column \"salary\" not found"},
		{"No row context", &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "id"}, Value: "id"}, nil, "evaluation requires row and schema context"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var val interface{}
			var err error
			if tt.name == "No row context" {
				val, err = Eval(tt.ident, nil, nil)
			} else {
				val, err = Eval(tt.ident, row, schema)
			}
			
			if tt.expectedErr != "" {
				if err == nil { t.Fatalf("expected error '%s', got nil", tt.expectedErr) }
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}
			if err != nil { t.Fatalf("Eval() returned unexpected error: %v", err) }
			if val != tt.expectedVal {
				t.Errorf("Eval() got = %v (%T), want = %v (%T)", val, val, tt.expectedVal, tt.expectedVal)
			}
		})
	}
}


func TestEvalInfixExpression(t *testing.T) {
	// Sample row and schema for tests involving identifiers
	colId := database.ColumnSchema{Name: "id", Type: database.BasicTypeInteger}
	colName := database.ColumnSchema{Name: "name", Type: database.BasicTypeText}
	colAge := database.ColumnSchema{Name: "age", Type: database.BasicTypeInteger}
	colSalary := database.ColumnSchema{Name: "salary", Type: database.BasicTypeNumeric} // Stored as float64
	colIsActive := database.ColumnSchema{Name: "is_active", Type: database.BasicTypeBoolean}
	colStartDate := database.ColumnSchema{Name: "start_date", Type: database.BasicTypeDate}
	
	testSchema := database.NewTableSchema("employees", []database.ColumnSchema{colId, colName, colAge, colSalary, colIsActive, colStartDate})
	
	dateVal, _ := time.Parse(DATE_LAYOUT, "2023-01-15")
	testRow := database.Row{int64(1), "Alice", int64(30), float64(50000.75), true, dateVal}

	// Helper to quickly create LiteralValue nodes
	intLit := func(val int64) *ast.LiteralValue { return &ast.LiteralValue{Token: token.Token{Type:token.INT, Literal:fmt.Sprint(val)}, Value:fmt.Sprint(val)} }
	strLit := func(val string) *ast.LiteralValue { return &ast.LiteralValue{Token: token.Token{Type:token.STRING, Literal:val}, Value:val} }
	boolLit := func(val bool) *ast.BooleanLiteral { 
		if val { return &ast.BooleanLiteral{Token: token.Token{Type:token.TRUE, Literal:"TRUE"}, Value:true} }
		return &ast.BooleanLiteral{Token: token.Token{Type:token.FALSE, Literal:"FALSE"}, Value:false}
	}
	ident := func(name string) *ast.Identifier { return &ast.Identifier{Token:token.Token{Type:token.IDENT, Literal:name}, Value:name}}

	tests := []struct {
		name        string
		node        *ast.InfixExpression
		row         database.Row // Can be nil if expression uses only literals
		schema      *database.TableSchema // Can be nil if expression uses only literals
		expectedVal interface{}
		expectedErr string
	}{
		// Integer Arithmetic
		{"10 + 5", &ast.InfixExpression{Left: intLit(10), Operator: "+", Right: intLit(5), Token: token.Token{Type:token.PLUS, Literal:"+"}}, nil, nil, int64(15), ""},
		{"10 - 5", &ast.InfixExpression{Left: intLit(10), Operator: "-", Right: intLit(5), Token: token.Token{Type:token.MINUS, Literal:"-"}}, nil, nil, int64(5), ""},
		{"10 * 5", &ast.InfixExpression{Left: intLit(10), Operator: "*", Right: intLit(5), Token: token.Token{Type:token.ASTERISK, Literal:"*"}}, nil, nil, int64(50), ""},
		{"10 / 5", &ast.InfixExpression{Left: intLit(10), Operator: "/", Right: intLit(5), Token: token.Token{Type:token.SLASH, Literal:"/"}}, nil, nil, int64(2), ""},
		{"10 / 0", &ast.InfixExpression{Left: intLit(10), Operator: "/", Right: intLit(0), Token: token.Token{Type:token.SLASH, Literal:"/"}}, nil, nil, nil, "division by zero"},
		
		// String Concatenation
		{"'a' + 'b'", &ast.InfixExpression{Left: strLit("a"), Operator: "+", Right: strLit("b"), Token:token.Token{Type:token.PLUS, Literal:"+"}}, nil, nil, "ab", ""},

		// Integer Comparisons
		{"10 = 5", &ast.InfixExpression{Left: intLit(10), Operator: "=", Right: intLit(5), Token:token.Token{Type:token.EQ, Literal:"="}}, nil, nil, false, ""},
		{"10 != 5", &ast.InfixExpression{Left: intLit(10), Operator: "!=", Right: intLit(5), Token:token.Token{Type:token.NEQ, Literal:"!="}}, nil, nil, true, ""},
		{"10 < 5", &ast.InfixExpression{Left: intLit(10), Operator: "<", Right: intLit(5), Token:token.Token{Type:token.LT, Literal:"<"}}, nil, nil, false, ""},
        {"age > 20", &ast.InfixExpression{Left: ident("age"), Operator: ">", Right: intLit(20), Token:token.Token{Type:token.GT, Literal:">"}}, testRow, testSchema, true, ""},


		// String Comparisons
		{"'a' = 'b'", &ast.InfixExpression{Left: strLit("a"), Operator: "=", Right: strLit("b"), Token:token.Token{Type:token.EQ, Literal:"="}}, nil, nil, false, ""},
		{"name = 'Alice'", &ast.InfixExpression{Left: ident("name"), Operator: "=", Right: strLit("Alice"), Token:token.Token{Type:token.EQ, Literal:"="}}, testRow, testSchema, true, ""},

		// Boolean Comparisons
		{"TRUE = FALSE", &ast.InfixExpression{Left: boolLit(true), Operator: "=", Right: boolLit(false), Token:token.Token{Type:token.EQ, Literal:"="}}, nil, nil, false, ""},
		{"is_active = TRUE", &ast.InfixExpression{Left: ident("is_active"), Operator: "=", Right: boolLit(true), Token:token.Token{Type:token.EQ, Literal:"="}}, testRow, testSchema, true, ""},
        {"is_active != FALSE", &ast.InfixExpression{Left: ident("is_active"), Operator: "!=", Right: boolLit(false), Token:token.Token{Type:token.NEQ, Literal:"!="}}, testRow, testSchema, true, ""},


		// Time/Date Comparisons (evaluator parses string literal to time.Time based on context)
		{"start_date = '2023-01-15'", &ast.InfixExpression{Left: ident("start_date"), Operator: "=", Right: strLit("2023-01-15"), Token:token.Token{Type:token.EQ, Literal:"="}}, testRow, testSchema, true, ""},
		{"start_date > '2022-12-31'", &ast.InfixExpression{Left: ident("start_date"), Operator: ">", Right: strLit("2022-12-31"), Token:token.Token{Type:token.GT, Literal:">"}}, testRow, testSchema, true, ""},
		{"'2023-01-01' < start_date", &ast.InfixExpression{Left: strLit("2023-01-01"), Operator: "<", Right: ident("start_date"), Token:token.Token{Type:token.LT, Literal:"<"}}, testRow, testSchema, true, ""},


		// Float/Numeric Arithmetic & Comparisons (evaluator promotes intLit to float64 if other side is float)
		{"salary + 1000", &ast.InfixExpression{Left: ident("salary"), Operator: "+", Right: intLit(1000), Token:token.Token{Type:token.PLUS, Literal:"+"}}, testRow, testSchema, float64(51000.75), ""},
		{"salary * 2", &ast.InfixExpression{Left: ident("salary"), Operator: "*", Right: intLit(2), Token:token.Token{Type:token.ASTERISK, Literal:"*"}}, testRow, testSchema, float64(100001.50), ""},
		{"salary > 50000", &ast.InfixExpression{Left: ident("salary"), Operator: ">", Right: intLit(50000), Token:token.Token{Type:token.GT, Literal:">"}}, testRow, testSchema, true, ""},
        {"salary = 50000.75", &ast.InfixExpression{Left: ident("salary"), Operator: "=", Right: &ast.LiteralValue{Token:token.Token{Type:token.STRING, Literal:"50000.75"}, Value:"50000.75"}, Token:token.Token{Type:token.EQ, Literal:"="}}, testRow, testSchema, true, ""},


		// NULL Handling (evaluator returns nil for these)
		{"id + NULL", &ast.InfixExpression{Left: ident("id"), Operator: "+", Right: &ast.Identifier{Value:"NULL"}, Token:token.Token{Type:token.PLUS, Literal:"+"}}, testRow, testSchema, nil, ""}, // Assuming NULL identifier results in nil value from evalIdentifier if not found
		{"name = NULL", &ast.InfixExpression{Left: ident("name"), Operator: "=", Right: &ast.Identifier{Value:"NULL"}, Token:token.Token{Type:token.EQ, Literal:"="}}, testRow, testSchema, nil, ""},
		
		// Type Mismatch
		{"1 + 'text'", &ast.InfixExpression{Left: intLit(1), Operator: "+", Right: strLit("text"), Token:token.Token{Type:token.PLUS, Literal:"+"}}, nil, nil, nil, "type mismatch"},
        {"'text' + 1", &ast.InfixExpression{Left: strLit("text"), Operator: "+", Right: intLit(1), Token:token.Token{Type:token.PLUS, Literal:"+"}}, nil, nil, nil, "type mismatch"},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := Eval(tt.node, tt.row, tt.schema)
			if tt.expectedErr != "" {
				if err == nil { t.Fatalf("expected error '%s', got nil", tt.expectedErr) }
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}
			if err != nil { t.Fatalf("Eval() returned unexpected error: %v", err) }
			
			// Comparing floats needs care due to precision. For this test, direct compare is fine.
			if fVal, ok := val.(float64); ok {
				if expFVal, okExp := tt.expectedVal.(float64); okExp {
					if fmt.Sprintf("%.2f", fVal) != fmt.Sprintf("%.2f", expFVal) { // Compare with 2 decimal places
						t.Errorf("Eval() float got = %v, want = %v", fVal, expFVal)
					}
					return // Done with float check
				}
			}

			if val != tt.expectedVal {
				t.Errorf("Eval() got = %v (%T), want = %v (%T)", val, val, tt.expectedVal, tt.expectedVal)
			}
		})
	}
}


func TestEvalContextualLiteralParsing(t *testing.T) {
	dateType := database.BasicTypeDate
	numericType := database.BasicTypeNumeric
    intType := database.BasicTypeInteger
    boolType := database.BasicTypeBoolean

	tests := []struct {
		name        string
		literalNode *ast.LiteralValue
		targetType  *database.BasicType
		expectedVal interface{}
		expectedErr string
	}{
		{"String to Date Valid", strLit("2024-03-10"), &dateType, time.Date(2024, 3, 10, 0,0,0,0, time.UTC), ""},
		{"String to Date Invalid", strLit("Mar 10, 2024"), &dateType, nil, "invalid date string"},
		{"String to Numeric Valid", strLit("123.45"), &numericType, float64(123.45), ""},
		{"String to Numeric Invalid", strLit("abc.def"), &numericType, nil, "invalid numeric string"},
        {"String to Integer Valid", strLit("42"), &intType, int64(42), ""},
        {"String to Integer Invalid", strLit("42.5"), &intType, nil, "invalid integer string"},
        {"String to Boolean Valid True", strLit("true"), &boolType, true, ""},
        {"String to Boolean Valid False Upper", strLit("FALSE"), &boolType, false, ""},
        {"String to Boolean Invalid", strLit("yes"), &boolType, nil, "invalid boolean string"},
		{"Int to Numeric", intLit(123), &numericType, float64(123), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// evalLiteralValue is not exported, so we test through Eval.
			// Eval itself calls evalLiteralValue, but it doesn't pass targetType directly.
			// The targetType logic is in evalInfixExpression's call to evalLiteralValue.
			// So, we need to simulate an infix expression to test this properly.
			
			// Construct a dummy InfixExpression:  dummy_ident OP literal
			// where dummy_ident has the targetType.
			// This is a bit indirect but tests the mechanism.
            var dummySchema *database.TableSchema
            var dummyRow database.Row
            
            var leftOperand ast.Expression
            if tt.targetType != nil {
                dummySchema = database.NewTableSchema("dummy", []database.ColumnSchema{{Name:"dummycol", Type:*tt.targetType}})
                // The actual value in dummyRow for dummycol doesn't matter for this test,
                // as we're focused on how the literal on the right is parsed.
                // But schema must be valid for Eval(ident, ...)
                if *tt.targetType == database.BasicTypeDate { dummyRow = database.Row{time.Now()} } else 
                if *tt.targetType == database.BasicTypeNumeric { dummyRow = database.Row{float64(0)} } else
                if *tt.targetType == database.BasicTypeInteger { dummyRow = database.Row{int64(0)} } else
                if *tt.targetType == database.BasicTypeBoolean { dummyRow = database.Row{false} } else
                { dummyRow = database.Row{nil} } // Default if type not critical for left side
                
                leftOperand = &ast.Identifier{Token:token.Token{Type:token.IDENT, Literal:"dummycol"}, Value:"dummycol"}
            } else {
                 // Should not happen for these tests, as targetType is key
                 t.Skip("Skipping test as targetType is nil, which is not the focus here.")
                 return
            }


			infixNode := &ast.InfixExpression{
				Left:     leftOperand, // So its type can be known
				Operator: "=", // Operator doesn't matter much here
				Right:    tt.literalNode,
				Token:    token.Token{Type: token.EQ, Literal: "="},
			}

			// We are interested in the evaluation of the right side (the literal)
			// The Eval(infixNode, ...) will call Eval(literalNode, ..., targetTypeFromLeft)
			// We can't directly get the result of just the literal part from outside.
			// So we check the error of the whole infix expression evaluation.
			// If there's an error, it should match tt.expectedErr if it's a parsing error for the literal.
			// If no error, we can't easily check the *converted* literal's value without more complex setup.

			// A more direct test of evalLiteralValue(node, targetType) would be better if it were public,
            // or if Eval could take an optional targetType hint for the top-level node.
            // For now, we test the effect via infix expression.
            
            // Let's simplify: test evalLiteralValue directly by making it temporarily public / testable
            // (or acknowledge this test is more of an integration test of that feature within infix eval)

			// For now, let's assume the logic in evalInfixExpression correctly calls evalLiteralValue with context.
			// This test will check if the overall infix evaluation fails as expected due to literal parsing.
			_, err := Eval(infixNode, dummyRow, dummySchema)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error '%s', got nil when evaluating infix with literal %s for target %v", 
                        tt.expectedErr, tt.literalNode.Value, *tt.targetType)
				}
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Eval() returned unexpected error: %v for literal %s and target %v", 
                        err, tt.literalNode.Value, *tt.targetType)
				}
				// Verifying the actual converted value (tt.expectedVal) is hard here
                // because we only get the result of the whole infix expression.
                // This test primarily checks error cases for contextual parsing.
			}
		})
	}
}

// Helper to create string literal AST node
func strLit(val string) *ast.LiteralValue {
	return &ast.LiteralValue{Token: token.Token{Type: token.STRING, Literal: val}, Value: val}
}

```
