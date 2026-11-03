package policy

import (
	"strconv"
	"strings"
	"time"

	"github.com/Auvis-Labs/auvis/types"
	"github.com/tidwall/gjson"
)

type EvalResult int

const (
	EvalFalse EvalResult = iota
	EvalTrue
	EvalIndeterminate
)

// MissingAttribute represents a variable reference that could not be resolved.
type MissingAttribute struct {
	Name string
}

// EvaluateCondition parses and evaluates the given condition string
// against the record JSON bytes and the request context.
// Returns (result, missingAttrName).
func EvaluateCondition(condition string, record []byte, reqCtx types.RequestContext) (EvalResult, string) {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return EvalTrue, "" // empty condition always evaluates to true
	}

	// Supported operators sorted by length descending to match correctly (e.g. != before !)
	operators := []string{"!=", "==", "<=", ">=", "<", ">", " contains ", " in "}
	var op string
	var parts []string

	for _, o := range operators {
		if idx := strings.Index(condition, o); idx != -1 {
			op = o
			parts = []string{
				strings.TrimSpace(condition[:idx]),
				strings.TrimSpace(condition[idx+len(o):]),
			}
			break
		}
	}

	if op == "" || len(parts) != 2 {
		// If condition is just a boolean literal like "true" or "false"
		if condition == "true" {
			return EvalTrue, ""
		}
		if condition == "false" {
			return EvalFalse, ""
		}
		// Invalid or unsupported condition format, default to true (fail-open)
		return EvalTrue, ""
	}

	lhsVal := resolveValue(parts[0], record, reqCtx)
	rhsVal := resolveValue(parts[1], record, reqCtx)

	if missing, ok := lhsVal.(MissingAttribute); ok {
		return EvalIndeterminate, missing.Name
	}
	if missing, ok := rhsVal.(MissingAttribute); ok {
		return EvalIndeterminate, missing.Name
	}

	if compareValues(lhsVal, op, rhsVal) {
		return EvalTrue, ""
	}
	return EvalFalse, ""
}

func resolveValue(expr string, record []byte, reqCtx types.RequestContext) interface{} {
	expr = strings.TrimSpace(expr)

	// 1. Literal string: 'value' or "value"
	if (strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) ||
		(strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) {
		return expr[1 : len(expr)-1]
	}

	// 2. Literal boolean
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	// 3. Literal number
	if val, err := strconv.ParseFloat(expr, 64); err == nil {
		return val
	}

	// 4. Record value: record.field
	if strings.HasPrefix(expr, "record.") {
		field := expr[7:]
		res := gjson.GetBytes(record, field)
		if !res.Exists() {
			return nil
		}
		switch res.Type {
		case gjson.True:
			return true
		case gjson.False:
			return false
		case gjson.Number:
			return res.Float()
		default:
			if res.IsArray() {
				var arr []interface{}
				for _, r := range res.Array() {
					arr = append(arr, r.Value())
				}
				return arr
			}
			return res.String()
		}
	}

	// 5. User value: user.field
	if strings.HasPrefix(expr, "user.") {
		field := strings.ToLower(expr[5:])
		if reqCtx.UserAttributes != nil {
			if val, ok := reqCtx.UserAttributes[field]; ok {
				// Try parsing as boolean or float if possible
				if val == "true" {
					return true
				}
				if val == "false" {
					return false
				}
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					return f
				}
				return val
			}
		}
		// Fallback to explicit RequestContext fields
		switch field {
		case "id", "userid":
			if reqCtx.UserID != "" {
				return reqCtx.UserID
			}
		case "role", "userrole":
			if reqCtx.UserRole != "" {
				return reqCtx.UserRole
			}
		}
		// Return MissingAttribute since the user field couldn't be resolved
		return MissingAttribute{Name: expr}
	}

	// 6. Time value: time.hour
	if strings.ToLower(expr) == "time.hour" {
		return float64(time.Now().Hour())
	}

	// 7. Array Literal
	if strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]") {
		jsonStr := strings.ReplaceAll(expr, "'", "\"")
		res := gjson.Parse(jsonStr)
		if res.IsArray() {
			var arr []interface{}
			for _, r := range res.Array() {
				arr = append(arr, r.Value())
			}
			return arr
		}
	}

	return expr
}

func compareValues(lhs interface{}, op string, rhs interface{}) bool {
	if lhs == nil || rhs == nil {
		if op == "!=" {
			return lhs != rhs
		}
		if op == "==" {
			return lhs == rhs
		}
		return false
	}

	// Try numeric comparison if both are float64
	lf, lok := lhs.(float64)
	rf, rok := rhs.(float64)
	if lok && rok {
		switch op {
		case "==":
			return lf == rf
		case "!=":
			return lf != rf
		case "<":
			return lf < rf
		case "<=":
			return lf <= rf
		case ">":
			return lf > rf
		case ">=":
			return lf >= rf
		}
		return false
	}

	// Try boolean comparison
	lb, lbok := lhs.(bool)
	rb, rbok := rhs.(bool)
	if lbok && rbok {
		switch op {
		case "==":
			return lb == rb
		case "!=":
			return lb != rb
		}
		return false
	}

	// Convert both to strings and compare
	ls := toString(lhs)
	rs := toString(rhs)

	switch op {
	case "==":
		return ls == rs
	case "!=":
		return ls != rs
	case "<":
		return ls < rs
	case "<=":
		return ls <= rs
	case ">":
		return ls > rs
	case ">=":
		return ls >= rs
	case " contains ":
		if lArr, ok := lhs.([]interface{}); ok {
			for _, el := range lArr {
				if compareValues(el, "==", rhs) {
					return true
				}
			}
			return false
		}
		return strings.Contains(ls, rs)
	case " in ":
		if rArr, ok := rhs.([]interface{}); ok {
			for _, el := range rArr {
				if compareValues(lhs, "==", el) {
					return true
				}
			}
			return false
		}
		return strings.Contains(rs, ls)
	}

	return false
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return ""
	}
}
