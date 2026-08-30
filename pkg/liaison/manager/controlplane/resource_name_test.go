package controlplane

import (
	"regexp"
	"testing"
)

func TestNormalizeResourceName_WhenProvided_KeepsTrimmedName(t *testing.T) {
	got, err := normalizeResourceName("  custom-name  ", "App")
	if err != nil {
		t.Fatalf("normalizeResourceName() error = %v", err)
	}
	if got != "custom-name" {
		t.Fatalf("normalizeResourceName() = %q, want %q", got, "custom-name")
	}
}

func TestNormalizeResourceName_WhenEmpty_GeneratesEightCharacterSuffix(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		match  string
	}{
		{name: "application", prefix: "App", match: `^App-[0-9a-f]{8}$`},
		{name: "connector", prefix: "Connector", match: `^Connector-[0-9a-f]{8}$`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeResourceName("", tc.prefix)
			if err != nil {
				t.Fatalf("normalizeResourceName() error = %v", err)
			}
			if !regexp.MustCompile(tc.match).MatchString(got) {
				t.Fatalf("normalizeResourceName() = %q, want match %q", got, tc.match)
			}
		})
	}
}

func TestNormalizeResourceName_WhenCalledRepeatedly_GeneratesDistinctNames(t *testing.T) {
	seen := make(map[string]struct{}, 32)
	for range 32 {
		name, err := normalizeResourceName("", "App")
		if err != nil {
			t.Fatalf("normalizeResourceName() error = %v", err)
		}
		if _, exists := seen[name]; exists {
			t.Fatalf("normalizeResourceName() generated duplicate %q", name)
		}
		seen[name] = struct{}{}
	}
}
