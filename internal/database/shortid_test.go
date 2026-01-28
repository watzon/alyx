package database

import (
	"regexp"
	"testing"
)

func TestGenerateShortID(t *testing.T) {
	id := GenerateShortID()

	if len(id) != 15 {
		t.Errorf("expected ID length 15, got %d", len(id))
	}

	pattern := regexp.MustCompile(`^[a-zA-Z0-9]{15}$`)
	if !pattern.MatchString(id) {
		t.Errorf("ID %q does not match expected pattern", id)
	}
}

func TestGenerateShortID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 10000

	for i := 0; i < iterations; i++ {
		id := GenerateShortID()
		if seen[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}
