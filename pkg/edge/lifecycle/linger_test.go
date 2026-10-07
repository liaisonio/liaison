package lifecycle

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestInstallLingerPreflight(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		err          error
	}{
		{"disabled", "no\n", nil},
		{"empty", "", nil},
		{"unknown", "unknown", nil},
		{"unavailable", "", errors.New("login manager unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, id, source, _ := testInstaller(t)
			original := i.run
			i.run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "/usr/bin/loginctl" {
					if strings.Join(args, " ") != "show-user 501 --property=Linger --value" {
						t.Fatal("unexpected user or mutating command", args)
					}
					return []byte(tc.output), tc.err
				}
				return original(ctx, name, args...)
			}
			_, err := i.installNew(context.Background(), id, source, InstallCredentials{"fixture.invalid:443", "key", "fixture"})
			if err == nil || !strings.Contains(err.Error(), "Linger") {
				t.Fatalf("expected actionable prerequisite error: %v", err)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatal("lost underlying error")
			}
			if _, err := os.Lstat(id.Home); !os.IsNotExist(err) {
				t.Fatal("preflight created installation files")
			}
		})
	}
}
