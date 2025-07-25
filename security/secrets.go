package security

import (
	"regexp"
)

// SecretPattern represents a pattern for detecting secrets
type SecretPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Description string
	Category    string
}

// RedactionResult represents the result of secret redaction
type RedactionResult struct {
	Content         string
	RedactionsCount int
	RedactedSecrets []RedactedSecret
}

// RedactedSecret represents a secret that was redacted
type RedactedSecret struct {
	Type        string
	Location    int    // Character position in original content
	Length      int    // Length of redacted content
	Context     string // Surrounding context
	Description string
}

// SecretDetector handles detection and redaction of secrets
type SecretDetector struct {
	patterns []SecretPattern
}
