export const webSSHHeartbeatIntervalMs = 25_000;
// Keep in sync with webSSHHeartbeatTTL. This is not a user inactivity limit.
export const webSSHHeartbeatTimeoutMs = 10 * 60_000;
export const webSSHHeartbeatProbeMs = 10_000;

export function startWebSSHHeartbeat(socket: WebSocket, onTimeout: () => void) {
  let lastAlive = Date.now();
  let probe: number | undefined;
  let stopped = false;
  const clearProbe = () => {
    if (probe !== undefined) window.clearTimeout(probe);
    probe = undefined;
  };
  const ping = () => {
    if (stopped || socket.readyState !== WebSocket.OPEN) return;
    // After a delayed timer, probe first and let queued messages run. Repeated
    // focus/online events must not keep extending an unanswered probe.
    if (Date.now() - lastAlive > webSSHHeartbeatTimeoutMs && probe === undefined) {
      probe = window.setTimeout(() => {
        probe = undefined;
        if (stopped || socket.readyState !== WebSocket.OPEN) return;
        if (Date.now() - lastAlive > webSSHHeartbeatTimeoutMs) {
          stop();
          onTimeout();
        }
      }, webSSHHeartbeatProbeMs);
    }
    try {
      socket.send(JSON.stringify({ type: 'ping' }));
    } catch {
      // The WebSocket error/close handler owns transport errors.
      stop();
    }
  };
  const visible = () => { if (document.visibilityState === 'visible') ping(); };
  const timer = window.setInterval(ping, webSSHHeartbeatIntervalMs);
  const stop = () => {
    stopped = true;
    window.clearInterval(timer);
    clearProbe();
    document.removeEventListener('visibilitychange', visible);
    window.removeEventListener('pageshow', ping);
    window.removeEventListener('online', ping);
  };
  document.addEventListener('visibilitychange', visible);
  window.addEventListener('pageshow', ping);
  window.addEventListener('online', ping);
  return {
    stop,
    alive() { if (!stopped) { lastAlive = Date.now(); clearProbe(); } },
  };
}
