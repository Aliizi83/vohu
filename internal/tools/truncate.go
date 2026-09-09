package tools

import "fmt"

// Truncate keeps the first and last half of maxChars from s, dropping the
// middle, so a tool never floods the model's context with a giant file or
// command output — but the caller still sees both where output started
// and how it ended (often the more informative half, e.g. a stack trace
// or a final error). A no-op if s already fits.
func Truncate(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}

	half := maxChars / 2
	omitted := len(s) - maxChars

	return fmt.Sprintf(
		"%s\n\n... [%d characters omitted] ...\n\n%s",
		s[:half],
		omitted,
		s[len(s)-half:],
	)
}
