import {Field, Input} from '@/components/ui';
import {useI18n} from '@/i18n';
import ApplicationProbe from '@/pages/App/ApplicationProbe';

export type ApplicationDraftValue = {name:string;address:string};
// Applications store a host and port, not a URL. Never silently discard a path,
// credentials, or TLS scheme from a pasted URL.
export function applicationTarget(address:string):{host:string;port:number}|undefined {
  const match=address.trim().match(/^(\[[0-9a-fA-F:.]+\]|[^\s/:?#@]+):(\d+)$/);
  if(!match)return;
  const port=Number(match[2]);
  if(!Number.isInteger(port)||port<1||port>65535)return;
  return {host:match[1].replace(/^\[|\]$/g,''),port};
}

export default function ApplicationDraft({edge,value,onChange,disabled}:{edge:string;value:ApplicationDraftValue;onChange:(value:ApplicationDraftValue)=>void;disabled:boolean}) {
  const {tr}=useI18n();
  const target=applicationTarget(value.address);
  return <fieldset className="liaison-llm-draft" disabled={disabled}>
    <Field label={tr('应用名称','Application name')}><Input value={value.name} placeholder={target?`App-${value.address.trim()}`:tr('选填','Optional')} onChange={e=>onChange({...value,name:e.target.value})}/></Field>
    <Field label={tr('应用地址','Application address')} required hint={tr('填写主机名或 IP 和端口，不含 URL 路径。地址从连接器所在设备访问，127.0.0.1 指该设备。','Enter a hostname or IP and port, without a URL path. The address is reached from the connector’s device. 127.0.0.1 refers to that device.')}>
      <Input required value={value.address} placeholder="127.0.0.1:8080" onChange={e=>onChange({...value,address:e.target.value})}/>
    </Field>
    <ApplicationProbe key={`${edge}:${value.address}`} edgeId={edge} host={target?.host} port={target?String(target.port):undefined} disabled={disabled||!target}/>
  </fieldset>;
}
