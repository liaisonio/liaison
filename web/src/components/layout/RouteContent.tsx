import { Component, Suspense, useEffect, useState, type ReactNode } from 'react';
import { useI18n } from '@/i18n';
import { Button, Notice } from '@/components/ui';

function RouteStatus({ failed = false }: { failed?: boolean }) {
  const { tr } = useI18n();
  const [slow, setSlow] = useState(false);
  useEffect(() => {
    const timer = window.setTimeout(() => setSlow(true), 10000);
    return () => window.clearTimeout(timer);
  }, []);
  if (failed || slow) return <Notice tone={failed ? 'danger' : 'info'}>
    <div role={failed ? 'alert' : 'status'}>
      {failed ? tr('页面加载失败，请重新加载。', 'Page failed to load. Please reload.') : tr('页面加载较慢，请稍候或重新加载。', 'Loading is taking longer. Please wait or reload.')}
      {' '}<Button onClick={() => window.location.reload()}>{tr('重新加载', 'Reload')}</Button>
    </div>
  </Notice>;
  return <div className="liaison-loading" role="status"><span aria-hidden />{tr('正在加载页面…', 'Loading page…')}</div>;
}

class RouteErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() { return this.state.failed ? <RouteStatus failed /> : this.props.children; }
}

export function RouteContent({ children }: { children: ReactNode }) {
  return <RouteErrorBoundary><Suspense fallback={<RouteStatus />}>{children}</Suspense></RouteErrorBoundary>;
}
