package evaluator

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"inmempg/ast"
	"inmempg/database"
	"inmempg/token"
)

const DATE_LAYOUT = "2006-01-02" // Standard SQL date format

// Eval evaluates an AST node.
func Eval(node ast.Node, row database.Row, schema *database.TableSchema) (interface{}, error) {
	switch n := node.(type) {
	case *ast.LiteralValue:
		return evalLiteralValue(n, nil) // No target type context here initially
	case *ast.BooleanLiteral:
		return n.Value, nil
	case *ast.Identifier:
		return evalIdentifier(n, row, schema)
	case *ast.InfixExpression:
		return evalInfixExpression(n, row, schema)
	default:
		return nil, fmt.Errorf("unsupported AST node type for evaluation: %T", node)
	}
}

// evalLiteralValue evaluates a literal, optionally trying to parse it into a targetType if provided.
func evalLiteralValue(lit *ast.LiteralValue, targetType *database.BasicType) (interface{}, error) {
	switch lit.Token.Type {
	case token.INT:
		val, err := strconv.ParseInt(lit.Value, 10, 64)
		if err != nil { return nil, fmt.Errorf("could not parse integer literal %q: %w", lit.Value, err) }
		// If target is NUMERIC (float64) or FLOAT, promote.
		if targetType != nil && (*targetType == database.BasicTypeNumeric || *targetType == database.BasicTypeFloat) {
			return float64(val), nil
		}
		return val, nil
	case token.STRING:
		// If a target type is known (e.g., from LHS of comparison), try to parse string into that type.
		if targetType != nil {
			switch *targetType {
			case database.BasicTypeDate:
				t, err := time.Parse(DATE_LAYOUT, lit.Value)
				if err != nil { return nil, fmt.Errorf("invalid date string %q: %w, expected format YYYY-MM-DD", lit.Value, err) }
				return t, nil
			case database.BasicTypeNumeric, database.BasicTypeFloat: // Try to parse string as float for numeric context
				f, err := strconv.ParseFloat(lit.Value, 64)
				if err != nil { return nil, fmt.Errorf("invalid numeric string %q: %w", lit.Value, err) }
				return f, nil
			case database.BasicTypeInteger: // Try to parse string as int for int context
				i, err := strconv.ParseInt(lit.Value, 10, 64)
				if err != nil { return nil, fmt.Errorf("invalid integer string %q: %w", lit.Value, err) }
				return i, nil
			case database.BasicTypeBoolean:
				lowerVal := strings.ToLower(lit.Value)
				if lowerVal == "true" { return true, nil }
				if lowerVal == "false" { return false, nil }
				return nil, fmt.Errorf("invalid boolean string %q (expected 'true' or 'false')", lit.Value)
			}
		}
		return lit.Value, nil // Return as string if no target type or not a special parseable string
	default:
		return nil, fmt.Errorf("unsupported literal value type for evaluation: %s", lit.Token.Type)
	}
}

func evalIdentifier(ident *ast.Identifier, row database.Row, schema *database.TableSchema) (interface{}, error) {
	if row == nil || schema == nil {
		return nil, fmt.Errorf("identifier %q evaluation requires row and schema context", ident.Value)
	}
	colNameLower := strings.ToLower(ident.Value)
	colSchema, ok := schema.ColMap[colNameLower]
	if !ok { return nil, fmt.Errorf("column %q not found in table schema %q", ident.Value, schema.Name) }
	colIndex := -1
	for i, sc := range schema.Columns {
		if strings.ToLower(sc.Name) == colNameLower { colIndex = i; break }
	}
	if colIndex == -1 { return nil, fmt.Errorf("internal error: column %q in ColMap but not in Columns list of schema %q", ident.Value, schema.Name) }
	if colIndex >= len(row) { return nil, fmt.Errorf("column index %d out of bounds for row with %d values (column %q)", colIndex, len(row), ident.Value) }
	return row[colIndex], nil
}

func evalInfixExpression(node *ast.InfixExpression, row database.Row, schema *database.TableSchema) (interface{}, error) {
	// Evaluate left side first to potentially get type context for the right side if it's a literal
	left, err := Eval(node.Left, row, schema)
	if err != nil { return nil, err }

	var rightTypeContext *database.BasicType = nil
	if lIdent, ok := node.Left.(*ast.Identifier); ok {
		if schema != nil { // Ensure schema is available
			colNameLower := strings.ToLower(lIdent.Value)
			if colSchema, exists := schema.ColMap[colNameLower]; exists {
				rightTypeContext = &colSchema.Type
			}
		}
	}


	// Special handling for LiteralValue on the right to use context from left
	var right interface{}
	if rLit, ok := node.Right.(*ast.LiteralValue); ok {
		right, err = evalLiteralValue(rLit, rightTypeContext)
	} else {
		right, err = Eval(node.Right, row, schema)
	}
	if err != nil { return nil, err }


	if left == nil || right == nil { // SQL NULL propagation
		// For IS NULL / IS NOT NULL, this logic would differ. For other comparisons, result is UNKNOWN (nil).
		return nil, nil 
	}

	// Type promotion and evaluation
	// Promote int64 to float64 if one is int64 and other is float64
	if (isInteger(left) && isFloat(right)) {
		left = float64(left.(int64))
	} else if (isFloat(left) && isInteger(right)) {
		right = float64(right.(int64))
	}


	switch lVal := left.(type) {
	case int64:
		rVal, ok := right.(int64)
		if !ok { return nil, fmt.Errorf("type mismatch for operator %s: expected %T for right operand, got %T (value: %v)", node.Operator, left, right, right) }
		return evalIntegerInfixExpression(node.Operator, lVal, rVal)
	case float64: // For NUMERIC and FLOAT
		rVal, ok := right.(float64)
		if !ok { return nil, fmt.Errorf("type mismatch for operator %s: expected %T for right operand, got %T (value: %v)", node.Operator, left, right, right) }
		return evalFloatInfixExpression(node.Operator, lVal, rVal)
	case string:
		rVal, ok := right.(string)
		if !ok { return nil, fmt.Errorf("type mismatch for operator %s: expected %T for right operand, got %T (value: %v)", node.Operator, left, right, right) }
		return evalStringInfixExpression(node.Operator, lVal, rVal)
	case bool:
		rVal, ok := right.(bool)
		if !ok { return nil, fmt.Errorf("type mismatch for operator %s: expected %T for right operand, got %T (value: %v)", node.Operator, left, right, right) }
		return evalBooleanInfixExpression(node.Operator, lVal, rVal)
	case time.Time:
		rVal, ok := right.(time.Time)
		if !ok { return nil, fmt.Errorf("type mismatch for operator %s: expected %T for right operand, got %T (value: %v)", node.Operator, left, right, right) }
		return evalTimeInfixExpression(node.Operator, lVal, rVal)
	default:
		return nil, fmt.Errorf("unsupported type for infix operation: %T %s ...", left, node.Operator)
	}
}
func isInteger(v interface{}) bool { _, ok := v.(int64); return ok }
func isFloat(v interface{}) bool { _, ok := v.(float64); return ok }


func evalIntegerInfixExpression(op string, left, right int64) (interface{}, error) { /* as before */ 
	switch op {
	case "+": return left + right, nil; case "-": return left - right, nil
	case "*": return left * right, nil
	case "/": if right == 0 { return nil, fmt.Errorf("division by zero") }; return left / right, nil
	case "=": return left == right, nil; case "!=", "<>": return left != right, nil
	case "<": return left < right, nil; case ">": return left > right, nil
	case "<=": return left <= right, nil; case ">=": return left >= right, nil
	default: return nil, fmt.Errorf("unknown integer operator: %s", op)
	}
}
func evalFloatInfixExpression(op string, left, right float64) (interface{}, error) { // For NUMERIC/FLOAT
	switch op {
	case "+": return left + right, nil; case "-": return left - right, nil
	case "*": return left * right, nil
	case "/": if right == 0.0 { return nil, fmt.Errorf("division by zero") }; return left / right, nil
	case "=": return left == right, nil; case "!=", "<>": return left != right, nil
	case "<": return left < right, nil; case ">": return left > right, nil
	case "<=": return left <= right, nil; case ">=": return left >= right, nil
	default: return nil, fmt.Errorf("unknown float operator: %s", op)
	}
}
func evalStringInfixExpression(op string, left, right string) (interface{}, error) { /* as before */ 
	switch op {
	case "+": return left + right, nil
	case "=": return left == right, nil; case "!=", "<>": return left != right, nil
	case "<": return left < right, nil; case ">": return left > right, nil
	case "<=": return left <= right, nil; case ">=": return left >= right, nil
	default: return nil, fmt.Errorf("unknown string operator: %s", op)
	}
}
func evalBooleanInfixExpression(op string, left, right bool) (interface{}, error) { /* as before */ 
	switch op {
	case "=": return left == right, nil; case "!=", "<>": return left != right, nil
	default: return nil, fmt.Errorf("unknown boolean operator: %s", op)
	}
}
func evalTimeInfixExpression(op string, left, right time.Time) (interface{}, error) { // New for DATE
	switch op {
	case "=": return left.Equal(right), nil
	case "!=", "<>": return !left.Equal(right), nil
	case "<": return left.Before(right), nil
	case ">": return left.After(right), nil
	case "<=": return left.Before(right) || left.Equal(right), nil
	case ">=": return left.After(right) || left.Equal(right), nil
	default: return nil, fmt.Errorf("unknown time/date operator: %s", op)
	}
}

```
