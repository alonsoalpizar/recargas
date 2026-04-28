package cedula

import (
	"regexp"
	"strings"
)

type Type string

const (
	Fisica   Type = "fisica"
	Juridica Type = "juridica"
	Dimex    Type = "dimex"
	Invalid  Type = "invalid"
)

var nonDigits = regexp.MustCompile(`[^0-9]`)

func Normalize(raw string) string {
	return nonDigits.ReplaceAllString(strings.TrimSpace(raw), "")
}

// Detect classifies a normalized cedula by length.
// Costa Rica: física = 9 digits, jurídica = 10 digits, DIMEX = 11-12 digits.
// Leading zero is rejected (no valid CR identifier starts with 0).
func Detect(clean string) Type {
	if clean == "" || strings.HasPrefix(clean, "0") {
		return Invalid
	}
	switch len(clean) {
	case 9:
		return Fisica
	case 10:
		return Juridica
	case 11, 12:
		return Dimex
	}
	return Invalid
}

func IsValid(clean string) bool {
	return Detect(clean) != Invalid
}
