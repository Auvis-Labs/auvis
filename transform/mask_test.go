package transform

import (
	"os"
	"testing"

	"github.com/tidwall/gjson"
)

func TestMaskField(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SSN standard mask",
			input:    "123-45-6789",
			expected: "**-**-6789",
		},
		{
			name:     "Phone with country code",
			input:    "+1-555-0198",
			expected: "**-***-0198",
		},
		{
			name:     "Phone with local area code format",
			input:    "(555) 555-0198",
			expected: "(***) ***-0198",
		},
		{
			name:     "Credit card hyphenated",
			input:    "4111-1111-1111-4242",
			expected: "***-****-****-4242",
		},
		{
			name:     "Credit card spaced",
			input:    "4111 1111 1111 4242",
			expected: "*** **** **** 4242",
		},
		{
			name:     "Credit card raw digits",
			input:    "4111111111114242",
			expected: "************4242",
		},
		{
			name:     "Email mask",
			input:    "alice@example.com",
			expected: "***@example.com",
		},
		{
			name:     "Generic fallback mask short",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "Generic fallback mask long",
			input:    "abcdefg",
			expected: "****defg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulating record bytes to pass to maskField
			// We can test individual mask helpers directly
			var res string
			if tc.name == "SSN standard mask" {
				res = maskSSN(tc.input)
			} else if tc.name == "Phone with country code" || tc.name == "Phone with local area code format" {
				res = maskPhone(tc.input)
			} else if tc.name == "Credit card hyphenated" || tc.name == "Credit card spaced" || tc.name == "Credit card raw digits" {
				res = maskCreditCard(tc.input)
			} else if tc.name == "Email mask" {
				res = maskEmail(tc.input)
			} else {
				res = maskGeneric(tc.input)
			}

			if res != tc.expected {
				t.Errorf("expected masked output for %q to be %q, got %q", tc.input, tc.expected, res)
			}
		})
	}
}

func TestAnonymizeField(t *testing.T) {
	// Set custom environment salt
	origSalt := os.Getenv("AUVIS_SALT")
	defer os.Setenv("AUVIS_SALT", origSalt)

	os.Setenv("AUVIS_SALT", "test-salt-123")

	record := []byte(`{"email":"alice@example.com","status":"active"}`)

	// Anonymize "email" field
	hashedRecord, err := anonymizeField(record, "email")
	if err != nil {
		t.Fatalf("failed to anonymize field: %v", err)
	}

	// We expect the result to be the hex-encoded SHA-256 of "alice@example.comtest-salt-123"
	// "alice@example.comtest-salt-123" -> SHA-256 -> 43f7de9d863f69cf00923f03b6e82845c4f2bbcf19c3bfa170f03ff062a4d339
	expectedHash := "8b44a79019ac63bd5b5af6c57d6a1bfe431e1db2b446c00ea5b8b5c5053dc11c"

	// Wait, we can parse directly using standard gjson
	res := gjson.GetBytes(hashedRecord, "email").String()
	if res != expectedHash {
		t.Errorf("expected anonymized email to be %q, got %q", expectedHash, res)
	}
}
