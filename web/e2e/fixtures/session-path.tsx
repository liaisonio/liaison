import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { useSessionPath } from '../../src/components/SessionReference/useSessionPath';
if (!import.meta.env.DEV) throw new Error('Development only');
function Preview() {
  const [handle, setHandle] = useState('');
  const [open, setOpen] = useState(false);
  const state = useSessionPath(handle, open, setOpen);
  const location = useLocation();
  return <><output>{location.pathname+location.search}</output>
    <button onClick={() => setHandle('fixture-secret-token')}>Connect</button>
    <button onClick={() => { setOpen(true); state.onAgentSessionReady('session_abcdef'); }}>Agent</button>
    <button onClick={state.closeAgent}>Close</button>
    <button onClick={state.toggleAgent}>Toggle</button>
    <pre>{JSON.stringify({matching: state.matching, open:state.agentOpen, agent:state.agentSessionId})}</pre>
  </>;
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><MemoryRouter initialEntries={['/webssh/2/connections/1?from=%2Fproxy']}><Preview /></MemoryRouter></React.StrictMode>);
