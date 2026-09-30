package claude

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/process"
)

// Driver owns the native process and bounded protocol transport. Probe callers
// disable tools; interactive sessions apply the shared approval policy.
type Driver struct {
	*Client
	cancel   context.CancelFunc
	waitDone chan struct{}
	once     sync.Once
}

func launchSpec(i discovery.Installation, project string) (process.Spec, error) {
	if i.Agent != "claude" {
		return process.Spec{}, errors.New("claude installation required")
	}
	// npm wrappers need an independently validated interpreter. Fail closed
	// until that path has native tests; do not invoke a shell or source profiles.
	switch strings.ToLower(filepath.Ext(i.ResolvedPath)) {
	case ".js", ".cjs", ".mjs", ".cmd", ".bat", ".ps1":
		return process.Spec{}, errors.New("native Claude Code executable required")
	}
	return process.Spec{Executable: i.Path, ResolvedExecutable: i.ResolvedPath, Directory: project, Args: []string{
		"--print", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--permission-prompt-tool", "stdio", "--safe-mode", "--tools", "",
		"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--setting-sources", "user", "--no-session-persistence",
	}}, nil
}

// StartProbe uses native user configuration without reading/copying credentials.
// Its lifetime context must span the whole driver, not just initialization.
func StartProbe(ctx context.Context, i discovery.Installation, project string) (*Driver, error) {
	spec, err := launchSpec(i, project)
	if err != nil {
		return nil, err
	}
	return startDriver(ctx, spec)
}

// startDriver 不导出任意参数入口。特殊工具仅在包内隔离测试中启用。
func startDriver(ctx context.Context, spec process.Spec) (*Driver, error) {
	return startBoundDriver(ctx, spec, "")
}

func startBoundDriver(ctx context.Context, spec process.Spec, expectedSession string) (*Driver, error) {
	lifetime, cancel := context.WithCancel(ctx)
	child, err := process.Start(lifetime, spec)
	if err != nil {
		cancel()
		return nil, errors.New("could not start Claude Code")
	}
	d := &Driver{Client: newClient(child.Input, child.Output, expectedSession), cancel: cancel, waitDone: make(chan struct{})}
	stop := context.AfterFunc(lifetime, func() { d.Client.fail(ErrClosed) })
	go func() {
		defer close(d.waitDone)
		defer stop()
		defer func() {
			if recover() != nil {
				d.Client.fail(ErrProtocol)
			}
		}()
		<-d.Client.Done()
		cancel()
		<-d.Client.readDone
		// Exit is expected after transport closure; raw process errors are not
		// exposed. Wait still reaps the child and cleans its owned process group.
		if waitErr := child.Wait(); waitErr != nil { /* closed probe */
		}
	}()
	initCtx, initCancel := context.WithTimeout(lifetime, 30*time.Second)
	defer initCancel()
	if err = d.Initialize(initCtx); err != nil {
		if closeErr := d.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return d, nil
}

func (d *Driver) Close() error {
	d.once.Do(func() { d.cancel(); d.Client.fail(ErrClosed) })
	<-d.waitDone
	return nil
}
