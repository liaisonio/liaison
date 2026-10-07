import { useEffect, useState } from 'react';
import { ArrowLeft, Keyboard, TerminalSquare } from 'lucide-react';
import { Button } from '@/components/ui';
import ActionMenu from '@/components/ui/ActionMenu';
import { useI18n } from '@/i18n';

function postToHost(value: string) {
  const bridge = (window as unknown as { webkit?: { messageHandlers?: { workspace?: { postMessage: (value: unknown) => void } } } }).webkit?.messageHandlers?.workspace;
  if (!bridge) return false;
  bridge.postMessage(value);
  return true;
}

type Props = {
  name: string; connected: boolean; connecting: boolean; active: boolean;
  reconnect: boolean; busy: boolean; filesOpen: boolean; canFiles: boolean; canAgent: boolean;
  canShell: boolean; shellOpen: boolean;
  onKeyboard: () => void; onAppearance: () => void; onBack: () => void; onConnect: () => void; onFiles: () => void;
  onAgent: () => void; onInfo: () => void; onShell: () => void; onDisconnect: () => void;
};

export default function MobileSessionBar(p: Props) {
  const { tr } = useI18n();
  const [integrated, setIntegrated] = useState(false);
  useEffect(() => {
    const root = document.documentElement;
    let requested = false;
    const update = () => {
      setIntegrated(root.dataset.liaisonChrome === 'integrated');
      const bridge = (window as unknown as { webkit?: { messageHandlers?: { workspace?: { postMessage: (value: unknown) => void } } } }).webkit?.messageHandlers?.workspace;
      if (!requested && root.hasAttribute('data-liaison-embedded') && bridge) {
        requested = true;
        bridge.postMessage('terminalChromeReady');
      }
    };
    const observer = new MutationObserver(update);
    observer.observe(root, { attributes: true, attributeFilter: ['data-liaison-chrome', 'data-liaison-embedded'] });
    update();
    return () => { observer.disconnect(); if (requested) postToHost('terminalChromeClosed'); };
  }, []);
  const status = p.connected ? tr('已连接', 'Connected') : p.connecting ? tr('连接中', 'Connecting') : p.reconnect ? tr('已断开', 'Disconnected') : p.active ? tr('待连接', 'Ready') : tr('不可用', 'Unavailable');
  return <footer className="webssh-mobile-session-bar" aria-label={tr('会话操作', 'Session controls')}>
    <Button variant="ghost" aria-label={tr('返回访问', 'Back to access')} onClick={() => { if (!integrated || !postToHost('close')) p.onBack(); }}><ArrowLeft size={18}/></Button>
    <button type="button" className="webssh-session-pill" onClick={p.onInfo} aria-label={`${p.name}, ${status}, ${tr('连接信息', 'Connection information')}`}>
      <i className={p.connected ? 'is-connected' : ''} aria-hidden="true"/><strong>{p.name}</strong><span className="sr-only" role="status">{status}</span>
    </button>
    {p.connected && <Button variant="ghost" aria-label={p.filesOpen ? tr('终端', 'Terminal') : tr('键盘', 'Keyboard')} onClick={p.filesOpen ? p.onFiles : p.onKeyboard}>{p.filesOpen ? <TerminalSquare size={18}/> : <Keyboard size={19}/>}</Button>}
    {!p.connected && <Button variant="primary" disabled={!p.active || p.busy || p.connecting} onClick={p.onConnect}>{p.connecting ? tr('连接中', 'Connecting') : p.reconnect ? tr('重新连接', 'Reconnect') : tr('连接', 'Connect')}</Button>}
    <ActionMenu menuClassName="webssh-touch-menu" placement="top" label={tr('更多终端操作', 'More terminal actions')} items={[
      ...(p.connected && p.canFiles ? [{ label: p.filesOpen ? tr('终端', 'Terminal') : tr('文件', 'Files'), onClick: p.onFiles }] : []),
      ...(p.connected && p.canAgent ? [{ label: 'Agent', onClick: p.onAgent }] : []),
      { label: tr('连接信息', 'Connection information'), onClick: p.onInfo },
      { label: tr('终端显示', 'Terminal appearance'), onClick: p.onAppearance },
      ...(integrated ? [{ label: tr('重新加载工作区', 'Reload workspace'), onClick: () => window.location.reload() }] : []),
      ...(p.connected && p.canShell ? [{ label: p.shellOpen ? tr('收起 Shell 助手', 'Hide Shell assistant') : tr('Shell 助手', 'Shell assistant'), onClick: p.onShell }] : []),
      ...(p.connected ? [{ label: tr('断开连接', 'Disconnect'), danger: true, onClick: p.onDisconnect }] : []),
    ]}/>
  </footer>;
}
