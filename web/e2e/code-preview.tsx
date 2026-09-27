import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import '../src/components/AgentWorkspace/index.less';
import { Transcript } from '../src/pages/EdgeAgent/Transcript';
import { resolveFileLink } from '../src/pages/EdgeAgent/fileLink';
import '../src/pages/EdgeAgent/index.less';
import '../src/pages/EdgeAgent/interaction.less';
import type { AgentSnapshot } from '../src/services/edgeAgent';
import '../src/styles/index.css';
Object.assign(window, { resolveFileLink });
function Fixture() {
  const [running, setRunning] = useState(true),
    [window, setWindow] = useState(0),
    [extra, setExtra] = useState(false),
    [available, setAvailable] = useState(true),
    [id, setID] = useState('s'.repeat(32));
  const session: AgentSnapshot = {
    version: 1,
    status: 'ok',
    session_id: id,
    project: '/workspace/project',
    window,
    running,
    closed: false,
    files_available: available,
    messages: [
      { role: 'user', text: 'Show the entrypoint [README](README.md) [README line](README.md#L3)' },
      {
        role: 'assistant',
        text: '[Entry](src/main.go#L3-L5) · [Absolute](/workspace/project/src/main.go:3) · [File URL](file:///workspace/project/src/main.go#L3) · [Colon](main.go:3) · [Directory](src/) · [Missing](missing.go) · [Large](large.go) · [Binary](binary.dat) · [Slow](slow.go) · [Outside](/etc/passwd) · [Traversal](../secret) · [Web](https://example.com) · [Unsafe](javascript:alert%281%29)',
      },
      ...(extra
        ? [{ role: 'assistant' as const, text: 'Continuing the same round' }]
        : []),
    ],
    activities: [
      {
        id: 'step',
        kind: 'commandExecution',
        status: extra || !running ? 'completed' : 'running',
        message_index: 1,
        duration_ms: 10,
        command: 'go test ./...',
        output: 'ok',
      },
    ],
  };
  return (
    <main style={{ padding: 16 }}>
      <button onClick={() => setExtra(true)}>Progress message</button>
      <button onClick={() => setRunning(false)}>Complete</button>
      <button
        onClick={() => {
          setWindow((n) => n + 1);
          setRunning(true);
          setExtra(false);
        }}
      >
        Next round
      </button>
      <button onClick={() => setAvailable(false)}>Offline</button>
      <button onClick={() => setID('t'.repeat(32))}>Switch session</button>
      <section
        className="edge-agent-workspace"
        style={{ height: 'calc(100dvh - 80px)' }}
      >
        <Transcript
          key={id}
          session={session}
          edge={6}
          accessID={'a'.repeat(32)}
        >
          <p>Empty</p>
        </Transcript>
      </section>
    </main>
  );
}
createRoot(document.getElementById('root')!).render(<Fixture />);
