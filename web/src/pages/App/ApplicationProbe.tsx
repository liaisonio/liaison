import {useEffect, useRef, useState} from 'react';
import {Button, Notice} from '@/components/ui';
import {request} from '@/api/client';
import {useI18n} from '@/i18n';
import './probe.less';

type Props = {edgeId?: string; host?: string; port?: string; applicationId?: number; disabled?: boolean};
type Result = {status: string; duration_ms?: number};

// The parent keys this component by the target, so stale results cannot survive edits.
export default function ApplicationProbe({edgeId, host, port, applicationId, disabled}: Props) {
  const {tr} = useI18n();
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<Result>();
  const controller = useRef<AbortController>();
  useEffect(() => () => { controller.current?.abort(); controller.current = undefined; }, []);
  const targetPort = Number(port || 443);
  const ready = !!applicationId || (!!edgeId && !!host?.trim() && Number.isInteger(targetPort) && targetPort > 0 && targetPort <= 65535);
  const labels: Record<string, string> = {
    reachable: tr('端口可达', 'TCP port reachable'),
    refused: tr('连接被拒绝，请检查服务是否启动和端口配置。', 'Connection refused. Check the service and port.'),
    timeout: tr('连接超时，请检查地址、防火墙和网络。', 'Connection timed out. Check the address, firewall and network.'),
    dns_error: tr('无法解析地址，请检查主机名。', 'Cannot resolve the address. Check the hostname.'),
    unreachable: tr('目标不可达，请检查地址和网络。', 'Target unreachable. Check the address and network.'),
    connector_offline: tr('连接器离线或已停用。', 'Connector is offline or disabled.'),
    edge_upgrade_required: tr('请升级连接器后再测试。', 'Upgrade the connector to test connectivity.'),
    busy: tr('探测繁忙，请稍后重试。', 'Too many probes. Try again shortly.'),
    probe_unavailable: tr('无法完成测试，请稍后重试。', 'Could not complete the test. Try again.'),
  };
  async function test() {
    if (!ready || busy) return;
    const abort = new AbortController();
    controller.current = abort;
    setBusy(true); setResult(undefined);
    const timeout = window.setTimeout(() => abort.abort(), 10000);
    try {
      const response = await request<{code: number; data?: Result}>('/api/v1/applications/probe', {
        method: 'POST', signal: abort.signal,
        data: applicationId ? {application_id: Number(applicationId)} : {edge_id: Number(edgeId), host: host?.trim(), port: targetPort},
      });
      if (!abort.signal.aborted) setResult(response.code === 200 && response.data ? response.data : {status: 'probe_unavailable'});
    } catch {
      if (!abort.signal.aborted) setResult({status: 'probe_unavailable'});
    } finally {
      window.clearTimeout(timeout);
      if (controller.current === abort) { setBusy(false); if (abort.signal.aborted) setResult({status: 'probe_unavailable'}); }
    }
  }
  return <div className="liaison-application-probe">
    <div className="liaison-application-probe-action"><Button loading={busy} disabled={disabled || !ready || busy} onClick={() => void test()}>{busy ? tr('测试中…', 'Testing…') : tr('测试连接', 'Test connection')}</Button>
      <span>{tr('仅检测 TCP 端口，不验证协议或账号。', 'Checks the TCP port only, not protocol or credentials.')}</span></div>
    <div aria-live="polite">{result && <Notice tone={result.status === 'reachable' ? 'success' : 'warning'}>{labels[result.status] || labels.probe_unavailable}{result.status === 'reachable' && typeof result.duration_ms === 'number' ? ` · ${result.duration_ms} ms` : ''}</Notice>}</div>
  </div>;
}
