package claude

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

func TestPersistentSpecUsesExactID(t *testing.T) {
	i := discovery.Installation{Agent: "claude", Path: "/bin/claude", ResolvedPath: "/bin/claude"}
	id, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	if !validSessionID(id) || id[14] != '4' {
		t.Fatal("invalid generated UUID")
	}
	for _, resume := range []bool{false, true} {
		spec, err := persistentSpec(i, "/project", id, resume)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(spec.Args, "|")
		want := "--session-id|" + id
		if resume {
			want = "--resume|" + id
		}
		if !strings.Contains(joined, want) || strings.Contains(joined, "--continue") || strings.Contains(joined, "--no-session-persistence") || strings.Contains(joined, "--fork-session") {
			t.Fatal("wrong persistence flags")
		}
	}
	for _, invalid := range []string{"", "latest", "/tmp/session.jsonl", "../session", "--continue", "00000000-0000-0000-0000-000000000000", strings.ToUpper(id)} {
		if _, err := persistentSpec(i, "/project", invalid, true); !errors.Is(err, ErrSessionID) {
			t.Fatal("invalid resume target accepted")
		}
	}
}

func TestNativeSessionIdentityMismatchCloses(t *testing.T) {
	for _, line := range []string{`{"type":"result","session_id":"other"}`, `{"type":"result"}`, `{"type":"assistant","session_id":"other"}`} {
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		c := newClient(inW, outR, "expected")
		if _, err := io.WriteString(outW, line+"\n"); err != nil {
			t.Fatal(err)
		}
		awaitClosed(t, c)
		if !errors.Is(c.Err(), ErrProtocol) {
			t.Fatal("foreign session accepted")
		}
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		if err := inR.Close(); err != nil {
			t.Fatal(err)
		}
		if err := outW.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckpointValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	if err := checkCheckpoint(path, false); err != nil {
		t.Fatal("new checkpoint unavailable")
	}
	if err := checkCheckpoint(path, true); !errors.Is(err, ErrCheckpoint) {
		t.Fatal("missing checkpoint resumed")
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkCheckpoint(path, true); err != nil {
		t.Fatal(err)
	}
	if err := checkCheckpoint(path, false); !errors.Is(err, ErrCheckpoint) {
		t.Fatal("existing session overwritten")
	}
	link := filepath.Join(root, "link.jsonl")
	if err := os.Symlink(path, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := checkCheckpoint(link, true); !errors.Is(err, ErrCheckpoint) {
		t.Fatal("symlink checkpoint accepted")
	}
	if err := checkCheckpoint(root, true); !errors.Is(err, ErrCheckpoint) {
		t.Fatal("directory checkpoint accepted")
	}
}

func TestTranscriptLocationBounded(t *testing.T) {
	id, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	project := filepath.Join(home, "my.project")
	path, err := transcriptPath(home, project, id)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != id+".jsonl" || !strings.HasPrefix(path, filepath.Join(home, ".claude", "projects")+string(filepath.Separator)) {
		t.Fatal("checkpoint escaped native store")
	}
	if _, err := transcriptPath(home, "relative", id); err == nil {
		t.Fatal("relative project accepted")
	}
	if _, err := transcriptPath(home, project+strings.Repeat("x", 201), id); err == nil {
		t.Fatal("unverified long-directory hashing accepted")
	}
}
