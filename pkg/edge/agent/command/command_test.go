package command

import (
	"context"
	"io"
	"testing"
)

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"--unknown"}, {"--agent-check"}, {"--agent-check", "--project", "relative"}, {"--agent-discover", "--model", "override"}, {"--agent-discover", "extra"}} {
		if err := Run(context.Background(), args, io.Discard); err == nil {
			t.Fatalf("accepted invalid arguments: %v", args)
		}
	}
}
