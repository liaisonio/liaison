package winservice

import (
	"strings"
	"testing"
)

func TestParseServiceOnlyAcceptsExplicitInstance(t *testing.T) {
	id := strings.Repeat("a", 32)
	name, args, err := Parse([]string{Flag, id, "-c", `C:\ProgramData\Liaison\edges\config.yaml`})
	if err != nil || name != "LiaisonEdge-"+id || len(args) != 2 {
		t.Fatalf("parse: %s %v %v", name, args, err)
	}
	for _, args := range [][]string{
		nil, {Flag}, {Flag, "legacy", "-c", "config"}, {Flag, "../" + id, "-c", "config"},
		{Flag, id, "-c", ""}, {Flag, id, "-c", "config", "extra"}, {"-c", id, "-c", "config"},
	} {
		if _, _, err := Parse(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}
