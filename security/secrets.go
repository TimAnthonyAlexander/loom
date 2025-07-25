package security

import (
	"fmt"
	"regexp"
	"strings"
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

// NewSecretDetector creates a new secret detector with comprehensive patterns

// Add all the secret patterns

// addDefaultPatterns adds comprehensive secret detection patterns
func (sd *SecretDetector) addDefaultPatterns() {
	// API Keys and Tokens
	sd.addPattern("API Key", `(?i)api[_-]?key[s]?[\s]*[:=][\s]*["']?([a-zA-Z0-9]{20,})["']?`, "API key detected", "api")
	sd.addPattern("Secret Key", `(?i)secret[_-]?key[s]?[\s]*[:=][\s]*["']?([a-zA-Z0-9]{20,})["']?`, "Secret key detected", "api")
	sd.addPattern("Access Token", `(?i)access[_-]?token[s]?[\s]*[:=][\s]*["']?([a-zA-Z0-9]{20,})["']?`, "Access token detected", "api")
	sd.addPattern("Bearer Token", `Bearer\s+([a-zA-Z0-9\-_\.]{20,})`, "Bearer token detected", "api")
	sd.addPattern("JWT Token", `eyJ[a-zA-Z0-9\-_]{20,}\.eyJ[a-zA-Z0-9\-_]{20,}\.[a-zA-Z0-9\-_]{20,}`, "JWT token detected", "api")

	// Passwords
	sd.addPattern("Password", `(?i)password[s]?[\s]*[:=][\s]*["']?([a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]{8,})["']?`, "Password detected", "auth")
	sd.addPattern("Pass", `(?i)pass[\s]*[:=][\s]*["']?([a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]{8,})["']?`, "Password detected", "auth")
	sd.addPattern("Passwd", `(?i)passwd[\s]*[:=][\s]*["']?([a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]{8,})["']?`, "Password detected", "auth")

	// Database Connection Strings
	sd.addPattern("MongoDB URI", `mongodb://[^@\s]+:[^@\s]+@[^\s]+`, "MongoDB connection string detected", "database")
	sd.addPattern("PostgreSQL URI", `postgres://[^@\s]+:[^@\s]+@[^\s]+`, "PostgreSQL connection string detected", "database")
	sd.addPattern("MySQL URI", `mysql://[^@\s]+:[^@\s]+@[^\s]+`, "MySQL connection string detected", "database")
	sd.addPattern("Redis URI", `redis://[^@\s]*:[^@\s]+@[^\s]+`, "Redis connection string detected", "database")

	// Cloud Provider Keys
	sd.addPattern("AWS Access Key", `AKIA[0-9A-Z]{16}`, "AWS access key detected", "cloud")
	sd.addPattern("AWS Secret Key", `(?i)aws[_-]?secret[_-]?access[_-]?key[\s]*[:=][\s]*["']?([a-zA-Z0-9/+=]{40})["']?`, "AWS secret access key detected", "cloud")
	sd.addPattern("Google API Key", `AIza[0-9A-Za-z\-_]{35}`, "Google API key detected", "cloud")
	sd.addPattern("Azure Key", `(?i)azure[_-]?key[\s]*[:=][\s]*["']?([a-zA-Z0-9]{40,})["']?`, "Azure key detected", "cloud")

	// Social/OAuth
	sd.addPattern("GitHub Token", `ghp_[a-zA-Z0-9]{36}`, "GitHub personal access token detected", "oauth")
	sd.addPattern("GitHub Classic Token", `[a-f0-9]{40}`, "GitHub classic token detected", "oauth")
	sd.addPattern("GitLab Token", `glpat-[a-zA-Z0-9\-_]{20}`, "GitLab personal access token detected", "oauth")
	sd.addPattern("Slack Token", `xox[baprs]-[0-9a-zA-Z\-]{10,}`, "Slack token detected", "oauth")
	sd.addPattern("Discord Token", `[MNO][a-zA-Z\d]{23}\.[\w-]{6}\.[\w-]{27}`, "Discord token detected", "oauth")

	// Generic Secrets
	sd.addPattern("Client Secret", `(?i)client[_-]?secret[\s]*[:=][\s]*["']?([a-zA-Z0-9]{20,})["']?`, "Client secret detected", "oauth")
	sd.addPattern("App Secret", `(?i)app[_-]?secret[\s]*[:=][\s]*["']?([a-zA-Z0-9]{20,})["']?`, "Application secret detected", "oauth")
	sd.addPattern("Private Key", `-----BEGIN [A-Z ]+PRIVATE KEY-----`, "Private key detected", "crypto")
	sd.addPattern("SSH Key", `ssh-rsa [A-Za-z0-9+/]+[=]{0,3}`, "SSH public key detected", "crypto")

	// Environment-specific
	sd.addPattern("Heroku API Key", `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`, "Heroku API key detected", "cloud")
	sd.addPattern("Stripe Key", `(?:r|s)k_live_[0-9a-zA-Z]{24}`, "Stripe live key detected", "payment")
	sd.addPattern("Twilio Key", `SK[a-z0-9]{32}`, "Twilio key detected", "api")
	sd.addPattern("SendGrid Key", `SG\.[a-zA-Z0-9\-_]{22}\.[a-zA-Z0-9\-_]{43}`, "SendGrid API key detected", "api")

	// Connection strings and URLs with credentials
	sd.addPattern("Generic URL with Credentials", `https?://[^@\s]+:[^@\s]+@[^\s]+`, "URL with embedded credentials detected", "url")
	sd.addPattern("FTP with Credentials", `ftp://[^@\s]+:[^@\s]+@[^\s]+`, "FTP URL with credentials detected", "url")

	// Certificates and hashes
	sd.addPattern("MD5 Hash", `[a-f0-9]{32}`, "MD5 hash detected", "hash")
	sd.addPattern("SHA1 Hash", `[a-f0-9]{40}`, "SHA1 hash detected", "hash")
	sd.addPattern("SHA256 Hash", `[a-f0-9]{64}`, "SHA256 hash detected", "hash")

	// Email/Phone (PII)
	sd.addPattern("Email", `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, "Email address detected", "pii")
	sd.addPattern("Phone", `\+?[1-9]\d{1,14}`, "Phone number detected", "pii")

	// Credit Card (basic pattern)
	sd.addPattern("Credit Card", `\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|3[0-9]{13}|6(?:011|5[0-9]{2})[0-9]{12})\b`, "Credit card number detected", "pii")
}

// addPattern adds a new secret detection pattern
func (sd *SecretDetector) addPattern(name, pattern, description, category string) {
	regex := regexp.MustCompile(pattern)
	sd.patterns = append(sd.patterns, SecretPattern{
		Name:        name,
		Pattern:     regex,
		Description: description,
		Category:    category,
	})
}

// DetectAndRedact detects and redacts secrets from content
func (sd *SecretDetector) DetectAndRedact(content string) *RedactionResult {
	result := &RedactionResult{
		Content:         content,
		RedactionsCount: 0,
		RedactedSecrets: make([]RedactedSecret, 0),
	}

	for _, pattern := range sd.patterns {
		matches := pattern.Pattern.FindAllStringSubmatch(result.Content, -1)
		indices := pattern.Pattern.FindAllStringSubmatchIndex(result.Content, -1)

		if len(matches) > 0 && len(indices) > 0 {
			for i, match := range matches {
				if len(match) > 1 && len(indices) > i {
					// Record the redaction
					redacted := RedactedSecret{
						Type:        pattern.Name,
						Location:    indices[i][2], // Start of first capture group
						Length:      len(match[1]),
						Context:     sd.getContext(result.Content, indices[i][2], 20),
						Description: pattern.Description,
					}
					result.RedactedSecrets = append(result.RedactedSecrets, redacted)

					// Replace the secret with [REDACTED]
					redactedText := fmt.Sprintf("[REDACTED_%s]", strings.ToUpper(strings.ReplaceAll(pattern.Category, " ", "_")))
					result.Content = strings.Replace(result.Content, match[1], redactedText, 1)
					result.RedactionsCount++
				}
			}
		}
	}

	return result
}

// getContext returns surrounding context for a secret location
func (sd *SecretDetector) getContext(content string, position, contextSize int) string {
	start := position - contextSize
	if start < 0 {
		start = 0
	}

	end := position + contextSize
	if end > len(content) {
		end = len(content)
	}

	return content[start:end]
}

// DetectOnly detects secrets without redacting them
func (sd *SecretDetector) DetectOnly(content string) []RedactedSecret {
	var detected []RedactedSecret

	for _, pattern := range sd.patterns {
		matches := pattern.Pattern.FindAllStringSubmatch(content, -1)
		indices := pattern.Pattern.FindAllStringSubmatchIndex(content, -1)

		if len(matches) > 0 && len(indices) > 0 {
			for i, match := range matches {
				if len(match) > 1 && len(indices) > i {
					secret := RedactedSecret{
						Type:        pattern.Name,
						Location:    indices[i][2],
						Length:      len(match[1]),
						Context:     sd.getContext(content, indices[i][2], 20),
						Description: pattern.Description,
					}
					detected = append(detected, secret)
				}
			}
		}
	}

	return detected
}

// GetPatternCount returns the number of registered patterns
func (sd *SecretDetector) GetPatternCount() int {
	return len(sd.patterns)
}

// GetPatterns returns all registered patterns (for debugging/info)
func (sd *SecretDetector) GetPatterns() []SecretPattern {
	return sd.patterns
}

// GetPatternsByCategory returns patterns filtered by category
func (sd *SecretDetector) GetPatternsByCategory(category string) []SecretPattern {
	var filtered []SecretPattern
	for _, pattern := range sd.patterns {
		if pattern.Category == category {
			filtered = append(filtered, pattern)
		}
	}
	return filtered
}

// GetCategories returns all unique categories
func (sd *SecretDetector) GetCategories() []string {
	categories := make(map[string]bool)
	for _, pattern := range sd.patterns {
		categories[pattern.Category] = true
	}

	var result []string
	for category := range categories {
		result = append(result, category)
	}
	return result
}

// FormatRedactionSummary creates a human-readable summary of redactions
func (result *RedactionResult) FormatRedactionSummary() string {
	if result.RedactionsCount == 0 {
		return "No secrets detected"
	}

	summary := fmt.Sprintf("Redacted %d secret(s):\n", result.RedactionsCount)

	// Group by type
	typeCount := make(map[string]int)
	for _, secret := range result.RedactedSecrets {
		typeCount[secret.Type]++
	}

	for secretType, count := range typeCount {
		if count == 1 {
			summary += fmt.Sprintf("- 1 %s\n", secretType)
		} else {
			summary += fmt.Sprintf("- %d %ss\n", count, secretType)
		}
	}

	return summary
}
