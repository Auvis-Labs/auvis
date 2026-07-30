package transform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// pseudonymizeField detects the value type and applies deterministic format-preserving HMAC pseudonymization.
func pseudonymizeField(record []byte, field string, seedOverride string) ([]byte, error) {
	val := gjson.GetBytes(record, field)
	if !val.Exists() {
		return record, nil
	}

	salt := seedOverride
	if salt == "" {
		salt = os.Getenv("AUVIS_SALT")
		if salt == "" {
			salt = "default-auvis-salt-value"
		}
	}

	if val.IsArray() {
		var out = record
		var err error
		var idx = 0
		val.ForEach(func(_, v gjson.Result) bool {
			elemPath := fmt.Sprintf("%s.%d", field, idx)
			pseudo := pseudonymizeValue(v, salt)
			out, err = sjson.SetBytes(out, elemPath, pseudo)
			idx++
			return err == nil
		})
		return out, err
	}

	pseudo := pseudonymizeValue(val, salt)
	return sjson.SetBytes(record, field, pseudo)
}

// pseudonymizeValue computes a keyed HMAC-SHA256 of value + salt and produces a deterministic pseudonymous replacement.
func pseudonymizeValue(val gjson.Result, salt string) interface{} {
	switch val.Type {
	case gjson.String:
		s := val.String()
		if s == "" {
			return ""
		}
		return pseudonymizeString(s, salt)
	case gjson.Number:
		// Convert number to string, pseudonymize, and derive a deterministic integer offset
		s := val.String()
		macHex := computeHMAC(s, salt)
		seed := hexToUint64(macHex)
		return int64((seed % 900000) + 100000)
	default:
		return val.Value()
	}
}

// pseudonymizeString inspects string structure and formats a realistic pseudonymous replacement.
func pseudonymizeString(val string, salt string) string {
	macHex := computeHMAC(val, salt)
	seed := hexToUint64(macHex)

	// 1. Email format preservation
	if strings.Contains(val, "@") {
		parts := strings.SplitN(val, "@", 2)
		domain := parts[1]
		if domain == "" {
			domain = "pseudo.auvis.internal"
		}
		return fmt.Sprintf("usr_%s@%s", macHex[:10], domain)
	}

	// 2. SSN format preservation (uses 999 reserved non-issued SSN prefix)
	if ssnPattern.MatchString(val) {
		middle := (seed / 10000) % 100
		lastFour := seed % 10000
		return fmt.Sprintf("999-%02d-%04d", middle, lastFour)
	}

	// 3. UUID format preservation
	if uuidPattern.MatchString(val) {
		return fmt.Sprintf("%s-%s-4%s-a%s-%s",
			macHex[:8],
			macHex[8:12],
			macHex[13:16],
			macHex[17:20],
			macHex[20:32],
		)
	}

	// 4. Prefixed ID format (e.g. usr_101, patient_8823, customer-99)
	if idx := strings.IndexAny(val, "_-"); idx > 0 && idx < len(val)-1 {
		prefix := val[:idx]
		return fmt.Sprintf("%s_psd_%s", prefix, macHex[:8])
	}

	// 5. Phone format preservation
	if phonePattern.MatchString(val) {
		lastFour := seed % 10000
		if strings.HasPrefix(val, "+234") {
			return fmt.Sprintf("+234-700-000-%04d", lastFour)
		}
		return fmt.Sprintf("+1-555-01%02d", (seed%90)+10)
	}

	// 6. Generic string fallback
	return fmt.Sprintf("Pseudo_%s", macHex[:8])
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of value using secret salt.
func computeHMAC(val string, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(val))
	return hex.EncodeToString(mac.Sum(nil))
}

// hexToUint64 parses the first 16 hex characters into a uint64 seed.
func hexToUint64(hexStr string) uint64 {
	if len(hexStr) < 16 {
		return 0
	}
	b, err := hex.DecodeString(hexStr[:16])
	if err != nil || len(b) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
