// Package command provides local diagnostics without enabling remote execution.
package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"

	"github.com/liaisonio/liaison/pkg/edge/agent/adapters/codex"
	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

// Run discovers existing executables, or starts a temporary owned app-server to
// check its protocol and authentication. It never sends a model inference call.
func Run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || (args[0] != "--agent-discover" && args[0] != "--agent-check") {
		return errors.New("unknown Agent diagnostic")
	}
	check := args[0] == "--agent-check"
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("path", "", "absolute installed Agent executable")
	project := flags.String("project", "", "absolute project directory for protocol check")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected Agent diagnostic arguments")
	}
	if check && !filepath.IsAbs(*project) {
		return errors.New("--agent-check requires an absolute --project directory")
	}
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		return err
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		return err
	}
	result, err := discovery.Find(ctx, platform, env, codex.Adapter{}, *path)
	if err != nil {
		return err
	}
	if !check {
		return json.NewEncoder(out).Encode(result)
	}
	if len(result.Installations) == 0 {
		return errors.New("Codex not found for the Edge execution account")
	}
	session, err := codex.Start(ctx, result.Installations[0], *project)
	if err != nil {
		return err
	}
	ready, checkErr := session.CheckAuthentication(ctx)
	closeErr := session.Close()
	if err := errors.Join(checkErr, closeErr); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(struct {
		Agent               string `json:"agent"`
		ProtocolReady       bool   `json:"protocol_ready"`
		AuthenticationReady bool   `json:"authentication_ready"`
		Stopped             bool   `json:"owned_instance_stopped"`
	}{"codex", true, ready, true})
}
