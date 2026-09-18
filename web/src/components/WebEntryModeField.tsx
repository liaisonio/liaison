import {useEffect, useState} from 'react';
import {Field, Select} from '@/components/ui';
import {useI18n} from '@/i18n';
import {webEntryCapabilities, type WebEntryMode} from '@/services/webEntry';

export function useWebEntryMode() {
  const [mode, setMode] = useState<WebEntryMode>('path');
  const [domainAvailable, setDomainAvailable] = useState(false);
  useEffect(() => {
    let active = true;
    void webEntryCapabilities().then(r => { if (active) setDomainAvailable(r.code === 200 && !!r.data?.domain); }).catch(() => {});
    return () => { active = false; };
  }, []);
  return {mode, setMode, domainAvailable};
}

export default function WebEntryModeField({mode, setMode, domainAvailable}: ReturnType<typeof useWebEntryMode>) {
  const {tr} = useI18n();
  return <Field label={tr('入口方式', 'Entry mode')} hint={mode === 'path'
    ? tr('使用独立路径。应用需支持子路径部署；不兼容时请选择端口或域名。仅用于可信应用。', 'Uses a dedicated path. Apps must support subpath deployment; otherwise use a port or domain. Trusted apps only.')
    : mode === 'domain' ? tr('使用已配置证书的独立子域名。', 'Uses a dedicated subdomain with the configured certificate.')
      : tr('为此访问分配独立监听端口。', 'Assigns a dedicated listening port to this access.')}>
    <Select value={mode} onChange={e => setMode(e.target.value as WebEntryMode)}>
      <option value="path">Path</option><option value="port">{tr('端口', 'Port')}</option>
      {domainAvailable && <option value="domain">{tr('域名', 'Domain')}</option>}
      {!domainAvailable && mode === 'domain' && <option value="domain" disabled>{tr('域名（不可用）', 'Domain (unavailable)')}</option>}
    </Select>
  </Field>;
}
