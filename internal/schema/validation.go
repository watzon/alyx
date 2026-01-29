package schema

import "fmt"

// ValidateIdentifier checks if a name is a valid SQL identifier.
// Valid identifiers must match: ^[a-z][a-z0-9_-]*$
func ValidateIdentifier(name string) error {
	if !IdentifierRegex.MatchString(name) {
		return fmt.Errorf("invalid identifier %q: must start with lowercase letter and contain only lowercase letters, numbers, underscores, and hyphens", name)
	}
	return nil
}
