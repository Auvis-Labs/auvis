package policy

import (
	"testing"

	"github.com/Auvis-Labs/auvis/types"
)

func TestEvaluateCondition(t *testing.T) {
	reqCtx := types.RequestContext{
		UserID:   "user-123",
		UserRole: "analyst",
		UserAttributes: map[string]string{
			"region":       "EU",
			"subscription": "enterprise",
		},
	}

	tests := []struct {
		name           string
		condition      string
		record         string
		expectedResult EvalResult
		expectedAttr   string
	}{
		{
			name:           "Empty condition always returns true",
			condition:      "",
			record:         `{"region": "EU"}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Record property string comparison matches",
			condition:      "record.region == 'EU'",
			record:         `{"region": "EU"}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Record property string comparison does not match",
			condition:      "record.region == 'US'",
			record:         `{"region": "EU"}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Record vs user property string comparison (not equal matches)",
			condition:      "record.region != user.region",
			record:         `{"region": "US"}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Record vs user property string comparison (not equal does not match)",
			condition:      "record.region != user.region",
			record:         `{"region": "EU"}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Record boolean comparison (consent true)",
			condition:      "record.consent == true",
			record:         `{"consent": true}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Record boolean comparison (consent false)",
			condition:      "record.consent == false",
			record:         `{"consent": false}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Record boolean comparison (consent false vs true)",
			condition:      "record.consent == true",
			record:         `{"consent": false}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Record numeric comparison (greater than)",
			condition:      "record.age > 18",
			record:         `{"age": 21}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Record numeric comparison (less than or equal)",
			condition:      "record.score <= 95.5",
			record:         `{"score": 95.5}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "User role comparison",
			condition:      "user.role == 'analyst'",
			record:         `{}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "User role comparison negative",
			condition:      "user.role != 'analyst'",
			record:         `{}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Invalid condition resolves gracefully to true (fail-open)",
			condition:      "invalid_syntax",
			record:         `{}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Non-existent record field comparison fails",
			condition:      "record.non_existent == 'value'",
			record:         `{}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Missing user attribute returns EvalIndeterminate",
			condition:      "record.region == user.missing_attr",
			record:         `{"region": "EU"}`,
			expectedResult: EvalIndeterminate,
			expectedAttr:   "user.missing_attr",
		},
		{
			name:           "String contains substring",
			condition:      "record.description contains 'urgent'",
			record:         `{"description": "this is an urgent request"}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "String contains substring negative",
			condition:      "record.description contains 'urgent'",
			record:         `{"description": "this is a normal request"}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Array contains element",
			condition:      "record.tags contains 'critical'",
			record:         `{"tags": ["info", "critical", "backend"]}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Array contains element negative",
			condition:      "record.tags contains 'critical'",
			record:         `{"tags": ["info", "backend"]}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "String in substring",
			condition:      "'urgent' in record.description",
			record:         `{"description": "this is an urgent request"}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Element in array literal",
			condition:      "user.role in ['admin', 'analyst']",
			record:         `{}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Element in array literal negative",
			condition:      "user.role in ['admin', 'manager']",
			record:         `{}`,
			expectedResult: EvalFalse,
		},
		{
			name:           "Element in record array",
			condition:      "'backend' in record.tags",
			record:         `{"tags": ["info", "critical", "backend"]}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Time hour check (basic evaluation true)",
			condition:      "time.hour >= 0",
			record:         `{}`,
			expectedResult: EvalTrue,
		},
		{
			name:           "Time hour check (basic evaluation false)",
			condition:      "time.hour < 0",
			record:         `{}`,
			expectedResult: EvalFalse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, attr := EvaluateCondition(tc.condition, []byte(tc.record), reqCtx)
			if res != tc.expectedResult {
				t.Errorf("expected EvaluateCondition(%q) result to be %v, got %v for record %s", tc.condition, tc.expectedResult, res, tc.record)
			}
			if tc.expectedResult == EvalIndeterminate && attr != tc.expectedAttr {
				t.Errorf("expected missing attribute name %q, got %q", tc.expectedAttr, attr)
			}
		})
	}
}
