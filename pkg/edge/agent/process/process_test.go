package process

import (
	"os"
	"testing"
)

func TestEnvironmentExcludesEdgeSecrets(t *testing.T) {
	t.Setenv("LIAISON_SECRET_KEY", "do-not-forward")
	t.Setenv("DATABASE_PASSWORD", "do-not-forward")
	t.Setenv("OPENAI_API_KEY", "local-agent-auth")
	found := false
	for _, value := range localEnvironment() {
		if value == "LIAISON_SECRET_KEY=do-not-forward" || value == "DATABASE_PASSWORD=do-not-forward" {
			t.Fatal("Edge credentials propagated")
		}
		if value == "OPENAI_API_KEY=local-agent-auth" {
			found = true
		}
	}
	if !found {
		t.Fatal("local Agent auth removed")
	}
}

func TestValidateRejectsRelativeProgram(t *testing.T) {
	if err := Validate(Spec{Executable: "codex", Directory: os.TempDir()}); err == nil {
		t.Fatal("relative executable accepted")
	}
}
