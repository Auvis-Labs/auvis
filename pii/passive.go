package pii

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// ExposedField represents a detected PHI/PII field that was NOT governed by an explicit YAML policy rule.
type ExposedField struct {
	Path      string `json:"path"`
	FieldType string `json:"field_type"`
	SampleVal string `json:"sample_val"`
}

var (
	regexSSN   = regexp.MustCompile(`^\d{3}-\d{2}-\d{4}$`)
	regexEmail = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	regexPhone = regexp.MustCompile(`^(\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}$`)
	regexDOB   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$|^\d{2}/\d{2}/\d{4}$`)
	regexCard  = regexp.MustCompile(`^(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13})$`)

	keyStemRegex = regexp.MustCompile(`(?i)\b(ssn|social_security|tax_id|ein|email|e_mail|dob|birth_date|bday|phone|mobile|telephone|npi|mrn)\b`)
)

// PassiveInspector scans unmasked fields in a JSON payload recursively for ungoverned PHI/PII.
type PassiveInspector struct{}

func NewPassiveInspector() *PassiveInspector {
	return &PassiveInspector{}
}

// ScanUngovernedPayload walks payload recursively and returns all exposed PHI fields not in governedFields.
func (p *PassiveInspector) ScanUngovernedPayload(payload map[string]any, governedFields map[string]bool) []ExposedField {
	if payload == nil {
		return nil
	}
	var exposed []ExposedField
	walkPayload(payload, "", governedFields, &exposed)
	return exposed
}

func walkPayload(val any, currentPath string, governed map[string]bool, exposed *[]ExposedField) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case map[string]any:
		for k, item := range v {
			fieldPath := k
			if currentPath != "" {
				fieldPath = currentPath + "." + k
			}
			if governed != nil && (governed[k] || governed[fieldPath]) {
				continue
			}
			walkPayload(item, fieldPath, governed, exposed)
		}
	case []any:
		for i, item := range v {
			indexPath := fmt.Sprintf("%s[%d]", currentPath, i)
			walkPayload(item, indexPath, governed, exposed)
		}
	case string:
		strVal := strings.TrimSpace(v)
		if strVal == "" {
			return
		}
		if fieldType, ok := classifyValue(currentPath, strVal); ok {
			maskedSample := maskString(strVal)
			*exposed = append(*exposed, ExposedField{
				Path:      currentPath,
				FieldType: fieldType,
				SampleVal: maskedSample,
			})
		}
	default:
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Map {
			for _, key := range rv.MapKeys() {
				kStr := fmt.Sprintf("%v", key.Interface())
				fieldPath := kStr
				if currentPath != "" {
					fieldPath = currentPath + "." + kStr
				}
				if governed != nil && (governed[kStr] || governed[fieldPath]) {
					continue
				}
				walkPayload(rv.MapIndex(key).Interface(), fieldPath, governed, exposed)
			}
		}
	}
}

func classifyValue(path, val string) (string, bool) {
	if regexSSN.MatchString(val) {
		return "ssn", true
	}
	if regexEmail.MatchString(val) {
		return "email", true
	}
	if regexPhone.MatchString(val) {
		return "phone", true
	}
	if regexCard.MatchString(val) {
		return "credit_card", true
	}
	if regexDOB.MatchString(val) {
		if isPastDate(val) {
			return "date_of_birth", true
		}
	}

	if len(val) >= 3 && path != "" {
		parts := strings.Split(path, ".")
		lastKey := parts[len(parts)-1]
		if keyStemRegex.MatchString(lastKey) {
			return lastKey, true
		}
	}

	return "", false
}

func isPastDate(val string) bool {
	t, err := time.Parse("2006-01-02", val)
	if err != nil {
		t, err = time.Parse("01/02/2006", val)
	}
	if err != nil {
		return false
	}
	return t.Before(time.Now())
}

func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

