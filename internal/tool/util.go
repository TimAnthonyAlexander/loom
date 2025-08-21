package tool

// SanitizeToolName converts a string into a safe tool name by keeping only
// ASCII letters, digits, and underscores. All other runes are mapped to '_'.
func SanitizeToolName(s string) string {
	b := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b = append(b, r)
		} else {
			b = append(b, '_')
		}
	}
	return string(b)
}
