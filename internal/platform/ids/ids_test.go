package ids

import (
	"regexp"
	"testing"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewV4IsLowercaseUUIDv4(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for range 1000 {
		id := NewV4()
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("NewV4() = %q, want lowercase UUID v4", id)
		}
		if seen[id] {
			t.Fatalf("NewV4() repeated %q", id)
		}
		seen[id] = true
	}
}
