import {
  Button,
  DataTable,
  Field,
  Input,
  Modal,
  Notice,
  Pager,
  Select,
  StatusPill,
} from '@/components/ui';
import { useI18n } from '@/i18n';
import {
  ideControl,
  ideID,
  ideListAll,
  ideRequest,
  ideRuntime,
  ideTheme,
  type IDEAccess,
  type IDEApplication,
  type IDECapabilities,
  type IDEConnector,
  type IDEInstallation,
  type IDEResult,
  type IDEInstance,
} from '@/services/webide';
import { Code2, Info, Plus, RefreshCw } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { RequestError } from '@/api/client';
import { EmptyAccessLayout } from '@/pages/Proxy/AccessEmptyState';
import './index.less';
import '@/pages/Proxy/connection.less';

export default function WebIDE({
  applications = false,
}: {
  applications?: boolean;
}) {
  const { tr } = useI18n();
  const [params, setParams] = useSearchParams();
  const [caps, setCaps] = useState<IDECapabilities>(),
    [apps, setApps] = useState<IDEApplication[]>([]),
    [accesses, setAccesses] = useState<IDEAccess[]>([]),
    [connectors, setConnectors] = useState<IDEConnector[]>([]);
  const requestedPage = Number(params.get('page') || 1);
  const page = Number.isInteger(requestedPage) && requestedPage > 0 && requestedPage <= 10000 ? requestedPage : 1;
  const nameFilter = (params.get('name') || '').slice(0,120);
  const searchName = useDebouncedValue(nameFilter.trim());
  const updateList = (values: Record<string, string | number>, replace = false) => setParams(previous => {
    const next = new URLSearchParams(previous);
    for (const [key,value] of Object.entries(values)) {
      if (!value || (key === 'page' && value === 1)) next.delete(key); else next.set(key,String(value));
    }
    return next;
  }, {replace});
  const setPage = (value: number) => updateList({page:value});
  const [total, setTotal] = useState(0),
    [revision, setRevision] = useState(0),
    [loading, setLoading] = useState(true),
    [error, setError] = useState('');
  const [appForm, setAppForm] = useState(false),
    [accessForm, setAccessForm] = useState(false),
    [busy, setBusy] = useState(false),
    [remove, setRemove] = useState<{ id: string; name: string }>();
  const access = accesses.find((a) => a.id === params.get('access'));
  const selectedApp = apps.find((a) => a.id === access?.application_id);
  const refresh = () => setRevision((v) => v + 1);
  const accessID = params.get('access');
  const [loadedAccessID, setLoadedAccessID] = useState('');
  const [instanceApp, setInstanceApp] = useState<IDEApplication>();
  const [stopOnDelete, setStopOnDelete] = useState(false);
  const [editing, setEditing] = useState<IDEAccess>();
  const deletingManagedAccess = !applications && accesses.find(e=>e.id===remove?.id)?.application_mode==='managed';
  useEffect(()=>setStopOnDelete(false),[remove?.id]);
  useEffect(()=>{if(applications&&params.get('new')==='1')setAppForm(true);},[applications,params]);
  useEffect(()=>{const selected=apps.find(a=>a.id===params.get('manage'));if(applications&&selected?.mode==='managed')setInstanceApp(selected);},[applications,apps,params]);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    void (async () => {
      const c = await ideRequest<IDECapabilities>(
        'capabilities',
        'GET',
        undefined,
        controller.signal,
      );
      if (controller.signal.aborted) return;
      setCaps(c);
      if (!c.enabled) return;
      if (!applications && accessID) {
        const detail = await ideRequest<{ access: IDEAccess; application: IDEApplication }>(
          `accesses/${encodeURIComponent(accessID)}`, 'GET', undefined, controller.signal,
        );
        if (!controller.signal.aborted) {
          setAccesses([detail.access]);
          setApps([detail.application]);
          setLoadedAccessID(accessID);
        }
        return;
      }
      const [a, e, d] = await Promise.all([
        applications ? ideRequest<{ items: IDEApplication[]; total: number }>(
          `applications?page=${applications ? page : 1}&page_size=${
            applications ? 20 : 100
          }`,
          'GET',
          undefined,
          controller.signal,
        ) : Promise.resolve({items:[] as IDEApplication[],total:0}),
        applications
          ? Promise.resolve({ items: [] as IDEAccess[], total: 0 })
          : ideRequest<{ items: IDEAccess[]; total: number }>(
              `accesses?page=${page}&page_size=20&name=${encodeURIComponent(searchName)}`,
              'GET',
              undefined,
              controller.signal,
            ),
        ideRequest<IDEConnector[]>(
          'connectors',
          'GET',
          undefined,
          controller.signal,
        ),
      ]);
      if (!controller.signal.aborted) {
        setApps(a.items);
        setAccesses(e.items);
        setConnectors(d);
        setTotal(applications ? a.total : e.total);
        const count = applications ? a.total : e.total;
        if (page > Math.max(1, Math.ceil(count / 20))) setPage(Math.max(1,Math.ceil(count/20)));
      }
    })()
      .catch(() => {
        if (!controller.signal.aborted)
          setError(
            tr(
              '无法加载 IDE，请检查权限或重试。',
              'Cannot load IDE. Check permissions or retry.',
            ),
          );
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [applications, page, revision, accessID, searchName]);
  async function deleteEntry() {
    if (!remove || busy) return;
    setBusy(true);
    setError('');
    try {
      if (!applications && stopOnDelete) {
        const entry = accesses.find(a => a.id === remove.id);
        if (!entry) throw Error();
        const instances = await ideRequest<IDEInstance[]>(`applications/${entry.application_id}/instances`);
        for (const instance of instances.filter(i => i.access_id === entry.id && i.status !== 'stopped')) {
          await ideRequest(`applications/${entry.application_id}/instances/${instance.id}`, 'POST', {action:'stop'});
        }
      }
      await ideRequest(
        `${applications ? 'applications' : 'accesses'}/${remove.id}`,
        'DELETE',
      );
      setRemove(undefined); setStopOnDelete(false);
      refresh();
    } catch {
      setError(
        tr(
          '删除失败。请检查连接器状态；删除应用前需停止实例并移除关联访问。',
          'Delete failed. Check the connector. Stop instances and remove linked accesses before deleting an application.',
        ),
      );
    } finally {
      setBusy(false);
    }
  }
  if (!applications && accessID) {
    if (loading || (!error && caps?.enabled && loadedAccessID !== accessID)) return <div role="status" className="webide-actions"><span className="ui-spinner" aria-hidden />{tr('正在打开 IDE…', 'Opening IDE…')}</div>;
    if (error || !access || !selectedApp) return <Notice tone="danger">
      {error || tr('IDE 访问不可用，请检查权限或配置。', 'IDE access is unavailable. Check permissions or configuration.')}
      <div className="webide-actions"><Button onClick={refresh}>{tr('重试', 'Retry')}</Button><Link to="/access/webide">{tr('返回访问列表', 'Back to accesses')}</Link></div>
    </Notice>;
    return <Workspace key={access.id} access={access} application={selectedApp} ready={!!caps?.ready} />;
  }
  return (
    <div className={`liaison-page-stack ${applications ? '' : 'liaison-access-list'}`}>
      {error && (
        <Notice tone="danger">
          {error}
          <Button onClick={refresh}>{tr('重试', 'Retry')}</Button>
        </Notice>
      )}
      {caps && !caps.enabled && (
        <Notice>
          {tr(
            'IDE 尚未启用，请联系管理员。',
            'IDE is disabled. Contact your administrator.',
          )}
        </Notice>
      )}
      {caps?.enabled && !caps.ready && (
        <Notice tone="warning">
          {tr(
            'IDE 入口尚未就绪，请检查 HTTPS 证书和服务地址。可以先登记应用和访问。',
            'IDE ingress is not ready. Check the HTTPS certificate and server URL. Applications and accesses can still be registered.',
          )}
        </Notice>
      )}
      {!applications && <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('访问名称','Access')}</span><input value={nameFilter} maxLength={120} placeholder={tr('输入访问名称','Access name')} onChange={e=>updateList({name:e.target.value,page:1},true)}/></label><div className="liaison-filter-actions">{nameFilter && <Button onClick={()=>updateList({name:'',page:1},true)}>{tr('重置','Reset')}</Button>}</div></div>}
      {access && selectedApp ? (
        <Workspace
          key={access.id}
          access={access}
          application={selectedApp}
          ready={!!caps?.ready}
        />
      ) : (
        <section className="liaison-list-panel">
          <header className="liaison-list-header">
            <h2>
              {applications
                ? tr('应用列表', 'Applications')
                : tr('访问列表', 'Access list')}
            </h2>
            <div className="webide-actions">
              <Button
                aria-label={tr('刷新', 'Refresh')}
                disabled={loading}
                onClick={refresh}
              >
                <RefreshCw size={16} />
              </Button>
              <Button
                variant="primary"
                disabled={!caps?.enabled || loading}
                onClick={() =>
                  applications ? setAppForm(true) : setAccessForm(true)
                }
              >
                <Plus size={14} />
                {applications
                  ? tr('新建应用', 'Create application')
                  : tr('新建访问', 'Create access')}
              </Button>
            </div>
          </header>
          {applications ? (
            <DataTable
              rows={apps}
              loading={loading}
              rowKey={(r) => r.id}
              emptyText={tr(
                '暂无 IDE 应用。登记设备上的安装或已有服务。',
                'No IDE applications. Register an installation or existing service.',
              )}
              columns={[
                {
                  key: 'name',
                  title: tr('应用名称', 'Application name'),
                  render: (r) => <span className="webide-list-label" title={r.name}>{r.name}</span>,
                },
                {
                  key: 'type',
                  title: tr('应用类型', 'Application type'),
                  render: () => (
                    <span className="liaison-inline-name">
                      <Code2 size={16} />
                      code-server
                    </span>
                  ),
                },
                {
                  key: 'device',
                  title: tr('设备', 'Device'),
                  render: (r) =>
                    connectors.find((c) => c.id === r.edge_id)?.name ||
                    String(r.edge_id),
                },
                {
                  key: 'mode',
                  title: tr('管理方式', 'Management'),
                  render: (r) =>
                    r.mode === 'external'
                      ? tr('已有服务', 'Existing service')
                      : tr('托管实例', 'Managed instances'),
                },
                {
                  key: 'action',
                  title: tr('操作', 'Actions'),
                  render: (r) => (
                    <span className="liaison-table-actions liaison-access-actions">
                    {r.mode === 'managed' && <button className="liaison-table-link" onClick={() => setInstanceApp(r)}>{tr('实例', 'Instances')}</button>}
                    <button
                      className="liaison-table-link is-danger"
                      onClick={() => setRemove(r)}
                    >
                      {tr('删除', 'Delete')}
                    </button>
                    </span>
                  ),
                },
              ]}
            />
          ) : !loading && !accesses.length ? (
            <EmptyAccessLayout icon={<Code2 size={20}/>} title={tr(searchName?'暂无匹配访问':'暂无 IDE 访问',searchName?'No matching access':'No IDE accesses')} description={tr(searchName?'可调整筛选条件或新建访问。':'新建访问，在 IDE 内选择设备上的文件夹。',searchName?'Adjust the filters or create an access.':'Create an access, then choose a folder on the device inside the IDE.')}/>
          ) : (
            <DataTable
              rows={accesses}
              loading={loading}
              rowKey={(r) => r.id}
              emptyText={tr(
                searchName ? '没有匹配的访问，请调整或重置筛选。' : '暂无 IDE 访问。新建访问以打开设备上的项目。',
                searchName ? 'No matching access. Change or reset the filter.' : 'No IDE accesses. Create an access to open a project on your device.',
              )}
              columns={[
                {
                  key: 'name',
                  title: tr('访问名称', 'Access name'),
                  render: (r) => <span className="webide-list-label" title={r.name}>{r.name}</span>,
                },
                {
                  key: 'type',
                  title: tr('访问类型', 'Access type'),
                  render: () => (
                    <span className="liaison-inline-name">
                      <Code2 size={16} />
                      IDE
                    </span>
                  ),
                },
                {
                  key: 'app',
                  title: tr('应用', 'Application'),
                  render: (r) =>
                    <span className="webide-list-label" title={r.application_name||r.application_id}>{r.application_name||'—'}</span>,
                },
                {
                  key: 'connector',
                  title: tr('连接器', 'Connector'),
                  render: (r) => {
                    return r.connector_name ? <span className="webide-list-label" title={r.connector_name}>{r.connector_name}</span> : '—';
                  },
                },
                {
                  key: 'connectorStatus',
                  title: tr('连接器状态', 'Connector status'),
                  render: (r) => {
                    return <StatusPill tone={r.connector_online ? 'success' : 'neutral'}>{r.connector_available ? r.connector_online ? tr('在线', 'Online') : tr('离线', 'Offline') : tr('不可用', 'Unavailable')}</StatusPill>;
                  },
                },
                {
                  key: 'status',
                  title: tr('访问状态', 'Access status'),
                  render: (r) => <StatusPill tone={r.enabled ? 'success' : 'neutral'}>{r.enabled ? tr('已启用', 'Enabled') : tr('已禁用', 'Disabled')}</StatusPill>,
                },
                {
                  key: 'action',
                  title: tr('操作', 'Actions'),
                  fixed: 'right',
                  width: '1%',
                  render: (r) => (
                    <span className="liaison-table-actions liaison-access-actions">
                      <button
                        className="liaison-table-link"
                        disabled={!r.enabled}
                        onClick={() => updateList({ access: r.id })}
                      >
                        {tr('去访问', 'Open')}
                      </button>
                      <button className="liaison-table-link" onClick={()=>setEditing(r)}>{tr('编辑','Edit')}</button>
                      <button
                        className="liaison-table-link is-danger"
                        onClick={() => setRemove(r)}
                      >
                        {tr('删除', 'Delete')}
                      </button>
                    </span>
                  ),
                },
              ]}
            />
          )}
          <Pager
            page={page}
            pageSize={20}
            total={total}
            onPageChange={setPage}
          />
        </section>
      )}
      {appForm && (
        <ApplicationForm
          connectors={connectors}
          onClose={() => setAppForm(false)}
          onSaved={() => {
            setAppForm(false);
            refresh();
          }}
        />
      )}
      {accessForm && (
        <AccessForm
          apps={apps}
          connectors={connectors}
          onClose={() => setAccessForm(false)}
          onSaved={() => {
            setAccessForm(false);
            refresh();
          }}
        />
      )}
      {editing && <EditAccess entry={editing} onClose={()=>setEditing(undefined)} onSaved={()=>{setEditing(undefined);refresh();}}/>}
      <Modal
        open={!!remove}
        title={applications ? tr('删除应用', 'Delete application') : tr('删除访问', 'Delete access')}
        onClose={() => {
          if (!busy) setRemove(undefined);
        }}
        footer={
          <>
            <Button disabled={busy} onClick={() => setRemove(undefined)}>
              {tr('取消', 'Cancel')}
            </Button>
            <Button
              variant="danger"
              loading={busy}
              onClick={() => void deleteEntry()}
            >
              {deletingManagedAccess && stopOnDelete ? tr('停止并删除访问', 'Stop and delete access') : applications ? tr('删除应用', 'Delete application') : tr('仅删除访问', 'Delete access only')}
            </Button>
          </>
        }
      >
        <p>
          {applications ? tr('删除应用前需停止实例并移除关联访问。不会删除设备上的项目文件。','Stop instances and remove linked accesses first. Project files on the device are preserved.') : tr(
            '不会删除项目文件。保留的实例可在「应用 → IDE → 实例」中恢复访问或停止。',
            'Project files are preserved. Manage retained instances under Applications → IDE → Instances to restore access or stop them.',
          )}
        </p>
        <strong>{remove?.name}</strong>
        {deletingManagedAccess && <fieldset className="webide-delete-options" disabled={busy}>
          <legend>{tr('如何处理后台实例？', 'What should happen to the background instances?')}</legend>
          <label><input type="radio" name="ide-delete-mode" checked={!stopOnDelete} onChange={()=>setStopOnDelete(false)}/><span><strong>{tr('仅删除访问，保留实例（默认）', 'Delete access only, keep instances (default)')}</strong><small>{tr('IDE、终端和后台任务继续运行。之后可在「应用 → IDE → 实例」重新创建访问。', 'The IDE, terminals and background tasks keep running. Recreate access later under Applications → IDE → Instances.')}</small></span></label>
          <label><input type="radio" name="ide-delete-mode" checked={stopOnDelete} onChange={()=>setStopOnDelete(true)}/><span><strong>{tr('停止实例并删除访问', 'Stop instances and delete access')}</strong><small>{tr('停止此访问关联的所有实例，将中断终端和后台任务。项目文件保留。', 'Stops all instances linked to this access, interrupting terminals and background tasks. Project files are preserved.')}</small></span></label>
        </fieldset>}
        {error && <Notice tone="danger">{error}</Notice>}
      </Modal>
      {instanceApp && <IDEInstances app={instanceApp} onClose={()=>{setInstanceApp(undefined);setParams(p=>{p.delete('manage');return p;},{replace:true});}} />}
    </div>
  );
}

function EditAccess({entry,onClose,onSaved}:{entry:IDEAccess;onClose:()=>void;onSaved:()=>void}) {
  const {tr}=useI18n();
  const [name,setName]=useState(entry.name),[enabled,setEnabled]=useState(entry.enabled);
  const [pending,setPending]=useState(false),[error,setError]=useState('');
  async function save(){
    if(pending||!name.trim())return;
    setPending(true);setError('');
    try{await ideRequest(`accesses/${entry.id}`,'PUT',{id:entry.id,name:name.trim(),application_id:entry.application_id,enabled,project:entry.project||''});onSaved();}
    catch{setError(tr('保存失败，请检查权限后重试。原配置未在页面中替换。','Save failed. Check permissions and retry. The displayed configuration is unchanged.'));}
    finally{setPending(false);}
  }
  return <Modal open title={tr('编辑访问','Edit access')} onClose={()=>{if(!pending)onClose();}} footer={<><Button disabled={pending} onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={!name.trim()} loading={pending} onClick={()=>void save()}>{tr('保存','Save')}</Button></>}>
    <div className="webide-form"><Field label={tr('访问名称','Access name')}><Input value={name} maxLength={120} disabled={pending} onChange={e=>setName(e.target.value)}/></Field>
      <Field label={tr('访问状态','Access status')} hint={tr('禁用后不能通过此入口访问 IDE，不会停止后台实例或删除文件。','Disabling blocks access through this entry without stopping instances or deleting files.')}><Select value={enabled?'enabled':'disabled'} disabled={pending} onChange={e=>setEnabled(e.target.value==='enabled')}><option value="enabled">{tr('已启用','Enabled')}</option><option value="disabled">{tr('已禁用','Disabled')}</option></Select></Field>
      {error&&<Notice tone="danger">{error}</Notice>}
    </div>
  </Modal>;
}

export function IDEInstances({app,onClose}:{app:IDEApplication;onClose:()=>void}) {
  const {tr}=useI18n();
  const [rows,setRows]=useState<(IDEInstance&{linked:boolean;access_name:string})[]>([]);
  const [loading,setLoading]=useState(true),[error,setError]=useState(''),[busy,setBusy]=useState(false);
  const [stop,setStop]=useState<IDEInstance>();
  const request = useRef<AbortController>();
  const load=async(quiet=false)=>{
    request.current?.abort();const controller=new AbortController();request.current=controller;
    if(!quiet)setLoading(true);setError('');
    try{const next=await ideRequest<(IDEInstance&{linked:boolean;access_name:string})[]>(`applications/${app.id}/instances`,'GET',undefined,controller.signal);if(!controller.signal.aborted)setRows(next);}
    catch{if(!controller.signal.aborted)setError(tr('无法获取实例，请检查连接器状态后重试。','Cannot load instances. Check the connector and retry.'));}
    finally{if(!controller.signal.aborted)setLoading(false);}
  };
  useEffect(()=>{void load();return()=>request.current?.abort();},[app.id]);
  const transitioning=rows.some(row=>row.status==='starting'||row.status==='stopping');
  useEffect(()=>{
    if(!transitioning||busy||error)return;
    const timer=window.setTimeout(()=>{void load(true);},3000);
    return()=>window.clearTimeout(timer);
  },[rows,busy,error]);
  useEffect(()=>{
    const refresh=()=>{if(!document.hidden&&!busy)void load(true);};
    window.addEventListener('focus',refresh);document.addEventListener('visibilitychange',refresh);
    return()=>{window.removeEventListener('focus',refresh);document.removeEventListener('visibilitychange',refresh);};
  },[app.id,busy]);
  const act=async(row:IDEInstance,action:'stop'|'recover')=>{setBusy(true);setError('');try{await ideRequest(`applications/${app.id}/instances/${row.id}`,'POST',{action});setStop(undefined);await load();}catch{setError(tr('操作失败，未删除项目文件，请重试。','Operation failed. Project files were not deleted. Retry.'));}finally{setBusy(false);}};
  const open=async(row:IDEInstance)=>{setBusy(true);setError('');try{
    let instance=row;
    if(row.status==='stopped'||row.status==='failed'){const result=await ideRuntime(row.access_id,'start',row.project);if(result.status!=='ok'||!result.instances?.[0])throw Error();instance=result.instances[0];}
    const target=await ideRequest<{url:string}>(`accesses/${row.access_id}/launch`,'POST',{instance_id:instance.id,project:row.project,theme:ideTheme()});
    const url=new URL(target.url);
    if(url.protocol!=='https:'||url.hostname!==window.location.hostname||url.username||url.password)throw Error();
    window.location.assign(url.href);
  }catch{setError(tr('无法打开实例，请检查访问是否启用及连接器状态。','Cannot open instance. Check that the access is enabled and the connector is online.'));}finally{setBusy(false);}};
  return <Modal open className="liaison-access-list" title={`${app.name} · ${tr('实例','Instances')}`} width={960} onClose={()=>{if(!busy)onClose();}} footer={<Button disabled={busy||loading} onClick={()=>void load()}>{tr('刷新','Refresh')}</Button>}>
    {error&&<Notice tone="danger">{error}</Notice>}
    <DataTable rows={rows} rowKey={r=>r.id} loading={loading} emptyText={tr('暂无托管实例','No managed instances')} columns={[
      {key:'state',title:tr('状态','Status'),render:r=><StatusPill tone={r.status==='running'?'success':r.status==='failed'?'danger':'neutral'}>{r.status==='running'?tr('运行中','Running'):r.status==='starting'?tr('启动中','Starting'):r.status==='stopped'?tr('已停止','Stopped'):r.status==='failed'?tr('启动失败','Start failed'):tr('未知','Unknown')}</StatusPill>},
      {key:'access',title:tr('关联访问','Linked access'),render:r=>r.linked?r.access_name:tr('未关联访问','No linked access')},
      {key:'started',title:tr('启动时间','Started'),render:r=>r.started_at&&!Number.isNaN(Date.parse(r.started_at))?new Date(r.started_at).toLocaleString(tr('zh-CN','en-US')):'—'},
      {key:'id',title:tr('实例','Instance'),render:r=><code title={r.id}>{r.id.slice(0,8)}</code>},
      {key:'actions',title:tr('操作','Actions'),fixed:'right',width:'1%',render:r=><span className="liaison-table-actions liaison-access-actions">{r.linked?<Button disabled={busy||r.status==='starting'} onClick={()=>void open(r)}>{r.status==='stopped'||r.status==='failed'?tr('启动并打开','Start and open'):tr('打开实例','Open instance')}</Button>:<Button disabled={busy} onClick={()=>void act(r,'recover')}>{tr('重新创建访问','Recreate access')}</Button>}{r.status!=='stopped'&&<Button disabled={busy} onClick={()=>setStop(r)}>{tr('停止','Stop')}</Button>}</span>},
    ]}/>
    {stop&&<Notice>{tr('停止将中断此实例的终端和后台任务，不删除项目文件。','Stopping interrupts terminals and background tasks in this instance. Project files are preserved.')}<div className="webide-actions"><Button disabled={busy} onClick={()=>setStop(undefined)}>{tr('取消','Cancel')}</Button><Button variant="danger" loading={busy} onClick={()=>void act(stop,'stop')}>{tr('确认停止','Confirm stop')}</Button></div></Notice>}
  </Modal>;
}

function ApplicationForm({
  connectors,
  onClose,
  onSaved,
}: {
  connectors: IDEConnector[];
  onClose: () => void;
  onSaved: (app: IDEApplication) => void;
}) {
  const { tr } = useI18n();
  const [id] = useState(ideID),
    [name, setName] = useState('code-server'),
    [edge, setEdge] = useState(''),
    [mode, setMode] = useState<'managed' | 'external'>('managed'),
    [port, setPort] = useState('8080'),
    [installation, setInstallation] = useState('');
  const [result, setResult] = useState<IDEResult>(),
    [pending, setPending] = useState(false),
    [error, setError] = useState(''),
    [installConfirm, setInstallConfirm] = useState(false);
  const generation = useRef(0);
  const [controlAction, setControlAction] = useState('');
  const controlAbort = useRef<AbortController>();
  useEffect(() => () => { generation.current++; controlAbort.current?.abort(); }, []);
  useEffect(() => {
    if (result?.status !== 'installing' || !edge) return;
    let disposed = false;
    let controller: AbortController | undefined;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      controller = new AbortController();
      const timeout = setTimeout(() => controller?.abort(), 30000);
      try {
        const value = await ideControl(Number(edge), 'discover', undefined, controller.signal);
        if (disposed) return;
        setError('');
        setResult(value);
        setInstallation(value.installations?.[0]?.id || '');
      } catch {
        if (!disposed) setError(tr('暂时无法读取安装状态，正在自动重试；设备上的任务不会因此取消。', 'Cannot read installation status. Retrying automatically; the device task is not cancelled.'));
      } finally {
        clearTimeout(timeout);
        if (!disposed) timer = setTimeout(() => void poll(), 3000);
      }
    }
    timer = setTimeout(() => void poll(), 3000);
    return () => {
      disposed = true;
      controller?.abort();
      clearTimeout(timer);
    };
  }, [edge, result?.status]);
  async function control(action: string) {
    if (pending || !edge) return;
    const epoch = ++generation.current;
    const controller = new AbortController();
    controlAbort.current?.abort();
    controlAbort.current = controller;
    const timeout = window.setTimeout(() => controller.abort(), 30000);
    setControlAction(action);
    setPending(true);
    setError('');
    setResult(undefined);
    setInstallation('');
    try {
      const r = await ideControl(Number(edge), action, undefined, controller.signal);
      if (epoch === generation.current) {
        setResult(r);
        setInstallation(r.installations?.[0]?.id || '');
        if (!['ok', 'installing', 'install_failed', 'non_root_required'].includes(r.status))
          setError(tr('连接器未能完成操作，请重试或检查连接器版本。', 'The connector could not complete this operation. Retry or check its version.'));
      }
    } catch {
      if (epoch === generation.current)
        setError(
          controller.signal.aborted ? tr(
            '连接器响应超时，请重新发现。若已提交安装，安装可能仍在设备上继续。',
            'The connector timed out. Retry discovery; a submitted installation may still be running on the device.',
          ) : tr(
            '连接器操作失败，请检查在线状态。',
            'Connector operation failed. Check whether it is online.',
          ),
        );
    } finally {
      window.clearTimeout(timeout);
      if (epoch === generation.current) { setPending(false); setControlAction(''); }
    }
  }
  async function save() {
    if (pending) return;
    setPending(true);
    setError('');
    try {
      onSaved(
        await ideRequest<IDEApplication>('applications', 'POST', {
          id,
          name,
          edge_id: Number(edge),
          mode,
          installation_id:
            mode === 'managed' ? installation : `external-${Number(port)}`,
          port: mode === 'external' ? Number(port) : 0,
        }),
      );
    } catch {
      setError(
        tr(
          '保存失败。保留当前表单重试不会重复创建。',
          'Save failed. Retrying this form will not create a duplicate.',
        ),
      );
    } finally {
      setPending(false);
    }
  }
  useEffect(() => {
    if (edge && mode === 'managed') void control('discover');
  }, [edge, mode]);
  return (
    <Modal
      open
      title={tr('新建 IDE 应用', 'Create IDE application')}
      onClose={() => {
        if (!pending) onClose();
      }}
      footer={
        <>
          <Button disabled={pending} onClick={onClose}>
            {tr('取消', 'Cancel')}
          </Button>
          <Button
            variant="primary"
            loading={pending && !controlAction}
            disabled={
              pending || !name.trim() || !edge || (mode === 'managed' && (!installation || !result?.can_launch || result?.status === 'installing')) || (mode === 'external' && (!Number.isInteger(Number(port)) || Number(port)<1 || Number(port)>65535))
            }
            onClick={() => void save()}
          >
            {tr('保存', 'Save')}
          </Button>
        </>
      }
    >
      <div className="webide-form">
        <Field label={tr('应用名称', 'Application name')}>
          <Input
            value={name}
            maxLength={120}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <Field label={tr('连接器', 'Connector')}>
          <Select
            disabled={pending}
            value={edge}
            onChange={(e) => {
              generation.current++;
              setEdge(e.target.value);
              setResult(undefined);
              setInstallation('');
              setInstallConfirm(false);
              setError('');
            }}
          >
            <option value="">{tr('选择连接器', 'Choose connector')}</option>
            {connectors.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
                {c.online ? '' : tr(' · 离线', ' · Offline')}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={tr('管理方式', 'Management')}>
          <Select
            value={mode}
            disabled={pending}
            onChange={(e) => setMode(e.target.value as 'managed' | 'external')}
          >
            <option value="managed">
              {tr('托管运行实例', 'Managed runtime')}
            </option>
            <option value="external">
              {tr('接入已有服务', 'Existing service')}
            </option>
          </Select>
        </Field>
        {mode === 'external' ? (
          <>
            <Field
              label={tr('设备本机 HTTP 端口', 'Device loopback HTTP port')}
              hint={tr(
                '保留已有服务的认证和配置，不接管进程。',
                'Existing authentication and configuration are preserved; Liaison does not manage this process.',
              )}
            >
              <Input
                type="number"
                min={1}
                max={65535}
                value={port}
                onChange={(e) => setPort(e.target.value)}
              />
            </Field>
          </>
        ) : (
          <>
            <div className="webide-actions">
              <Button
                disabled={!edge || pending}
                loading={pending && controlAction === 'discover'}
                onClick={() => void control('discover')}
              >
                {pending && controlAction === 'discover' ? tr('正在查找…', 'Searching…') : tr('发现安装', 'Discover installation')}
              </Button>
              {edge && !pending && result && (!result.installations?.length || result.status === 'installing') && <Button
                disabled={!edge || pending || result?.status === 'installing'}
                loading={controlAction === 'install' || result?.status === 'installing'}
                onClick={() => setInstallConfirm(true)}
              >
                {controlAction === 'install' ? tr('正在提交…', 'Submitting…') : result?.status === 'installing' ? tr('正在安装…', 'Installing…') : tr('安装 code-server', 'Install code-server')}
              </Button>}
            </div>
            {installConfirm && (
              <Notice tone="warning">
                {tr('将在所选设备上下载并校验 code-server，不替换已有安装。', 'Download and verify code-server on the selected device without replacing existing installations.')}
                <div className="webide-actions">
                  <Button onClick={() => setInstallConfirm(false)}>{tr('取消', 'Cancel')}</Button>
                  <Button variant="primary" onClick={() => { setInstallConfirm(false); void control('install'); }}>{tr('确认安装', 'Confirm installation')}</Button>
                </div>
              </Notice>
            )}
            {controlAction === 'install' && <Notice><span role="status">{tr('正在向设备提交安装请求…', 'Submitting the installation request to the device…')}</span></Notice>}
            {!pending && !error && result?.status === 'ok' && result.can_launch && !result.installations?.length && (
              <Notice><span role="status">{tr('未找到 code-server。请点击「安装 code-server」，完成后即可保存。', 'No code-server installation found. Click “Install code-server”; save after installation completes.')}</span></Notice>
            )}
            {!edge && <small>{tr('请先选择运行 IDE 的连接器。', 'Select the connector that will run IDE first.')}</small>}
            {result?.status === 'installing' && (
              <Notice>
                <span role="status">
                {tr(
                  '正在下载、校验并解压，完成后自动更新。关闭窗口不会取消安装。',
                  'Downloading, verifying and extracting. Status updates automatically. Closing this dialog does not cancel installation.',
                )}
                </span>
              </Notice>
            )}
            {result?.status === 'ok' && !!result.installations?.length && (
              <Notice tone="success"><span role="status">{tr('code-server 已就绪，已自动选择安装。点击「保存」创建应用。', 'code-server is ready and selected. Click Save to create the application.')}</span></Notice>
            )}
            {result?.status === 'install_failed' && (
              <Notice tone="danger">
                {tr(
                  '安装失败，请检查设备网络、磁盘与权限后重试。',
                  'Installation failed. Check device network, disk space and permissions.',
                )}
              </Notice>
            )}
            {result && !result.can_launch && (
              <Notice tone="warning">
                {tr(
                  '此连接器不能启动 IDE。需要支持的平台及非 root 开发账号。',
                  'This connector cannot launch IDEs. A supported platform and a non-root development account are required.',
                )}
              </Notice>
            )}
            {!!result?.installations?.length && (
              <Field label={tr('code-server 安装', 'code-server installation')}>
                <div className="webide-installation">
                {result.installations.length === 1 ? <span>code-server{result.installations[0].version ? ` · ${result.installations[0].version}` : ''}</span> : (
                <Select
                  value={installation}
                  onChange={(e) => setInstallation(e.target.value)}
                >
                  {result.installations.map((i: IDEInstallation, index: number) => (
                    <option key={i.id} value={i.id}>
                      code-server · {tr('安装', 'Installation')} {index + 1}
                      {i.version ? ` · ${i.version}` : ''}
                    </option>
                  ))}
                </Select>
                )}
                <span className="webide-installation-info" tabIndex={0} role="img" aria-label={`${tr('安装路径', 'Installation path')}: ${result.installations.find((i) => i.id === installation)?.path || ''}`}>
                  <Info size={14} aria-hidden="true" />
                  <span className="webide-installation-tooltip" aria-hidden="true">{result.installations.find((i) => i.id === installation)?.path}</span>
                </span>
                </div>
              </Field>
            )}
          </>
        )}
        <Notice>
          {tr(
            'IDE 的终端和扩展拥有设备开发账号的权限。仅向受信任的使用者开放。',
            'IDE terminals and extensions have the development account’s permissions. Grant access only to trusted users.',
          )}
        </Notice>
        {error && <Notice tone="danger">{error}</Notice>}
      </div>
    </Modal>
  );
}

function AccessForm({
  apps,
  connectors,
  onClose,
  onSaved,
}: {
  apps: IDEApplication[];
  connectors: IDEConnector[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const { tr } = useI18n();
  const [id] = useState(ideID),
    [name, setName] = useState('IDE'),
    [app, setApp] = useState(apps[0]?.id || ''),
    [extra, setExtra] = useState<IDEApplication[]>([]),
    [create, setCreate] = useState(false),
    [pending, setPending] = useState(false),
    [error, setError] = useState('');
  const [choices, setChoices] = useState(apps);
  const [loadingApps, setLoadingApps] = useState(true);
  const [appsError, setAppsError] = useState(false);
  const [appsRevision, setAppsRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 30000);
    let disposed = false;
    setLoadingApps(true);
    setAppsError(false);
    void (async () => {
      const items = await ideListAll<IDEApplication>('applications',controller.signal);
      if (disposed) return;
      setChoices(items);
      setApp((selected) => items.some((a) => a.id === selected) || extra.some((a) => a.id === selected) ? selected : items[0]?.id || '');
    })().catch(() => { if (!disposed) setAppsError(true); })
      .finally(() => { window.clearTimeout(timeout); if (!disposed) setLoadingApps(false); });
    return () => { disposed = true; controller.abort(); window.clearTimeout(timeout); };
  }, [appsRevision]);
  const available = [...new Map([...choices, ...extra].map((a) => [a.id, a])).values()];
  const selected = available.find((a) => a.id === app);
  const connector = connectors.find((c) => c.id === selected?.edge_id);
  async function save() {
    if (pending || loadingApps || appsError || !selected || !name.trim()) return;
    setPending(true);
    setError('');
    try {
      await ideRequest('accesses', 'POST', {
        id,
        name,
        application_id: app,
        enabled: true,
      });
      onSaved();
    } catch {
      setError(
        tr(
          '保存失败，请检查权限后重试。',
          'Save failed. Check permissions and retry.',
        ),
      );
    } finally {
      setPending(false);
    }
  }
  if (create)
    return (
      <ApplicationForm
        connectors={connectors}
        onClose={() => setCreate(false)}
        onSaved={(a) => {
          setExtra((v) => [...v, a]);
          setApp(a.id);
          setCreate(false);
        }}
      />
    );
  return (
    <Modal
      open
      title={tr('新建访问', 'Create access')}
      onClose={() => {
        if (!pending) onClose();
      }}
      footer={
        <>
          <Button disabled={pending} onClick={onClose}>
            {tr('取消', 'Cancel')}
          </Button>
          <Button
            variant="primary"
            loading={pending}
            disabled={pending || loadingApps || appsError || !selected || !name.trim()}
            onClick={() => void save()}
          >
            {tr('保存', 'Save')}
          </Button>
        </>
      }
    >
      <div className="webide-form">
        <Field label={tr('访问名称', 'Access name')}>
          <Input
            maxLength={120}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <Field label={tr('已有应用', 'Existing application')}>
          <span className="webide-select-row">
          <Select
            value={app}
            disabled={pending || loadingApps || appsError}
            onChange={(e) => {
              if (e.target.value === 'new') {
                setCreate(true);
                return;
              }
              setApp(e.target.value);
            }}
          >
            <option value="">{loadingApps ? tr('正在加载应用…', 'Loading applications…') : !available.length ? tr('暂无已保存的 IDE 应用', 'No saved IDE applications') : tr('选择应用', 'Choose application')}</option>
            {available.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name} · {connectors.find((c) => c.id === a.edge_id)?.name || `#${a.edge_id}`}
              </option>
            ))}
            <option value="new">
              {tr('新建应用…', 'Create application…')}
            </option>
          </Select>
          <Button aria-label={tr('刷新应用','Refresh applications')} title={tr('刷新应用','Refresh applications')} disabled={pending || loadingApps} loading={loadingApps} onClick={() => setAppsRevision((v) => v + 1)}><RefreshCw size={14}/></Button>
          </span>
        </Field>
        <p className="webide-list-muted">{tr('进入 IDE 后选择设备上的文件夹。', 'Choose a folder on the device after opening the IDE.')}</p>
        {appsError && <Notice tone="danger">{tr('应用列表加载失败，请刷新重试。未修改当前选择。', 'Cannot load applications. Refresh to retry; your selection is unchanged.')}</Notice>}
        {!loadingApps && !appsError && !available.length && <Notice>{tr('暂无 IDE 应用，请在上方下拉框中选择「新建应用…」。', 'No IDE applications yet. Select “Create application…” from the dropdown above.')}</Notice>}
        {!loadingApps && !appsError && selected && <div className="webide-list-muted">
          <div>{tr('连接器', 'Connector')}：{connector?.name || `#${selected.edge_id}`}{connector ? (connector.online ? tr(' · 在线', ' · Online') : tr(' · 离线', ' · Offline')) : ''}</div>
          <div>{tr('管理方式', 'Management')}：{selected.mode === 'managed' ? tr('托管运行实例', 'Managed runtime') : tr('接入已有服务', 'Existing service')}{selected.mode === 'external' ? ` · 127.0.0.1:${selected.port}` : ''}</div>
        </div>}
        {error && <Notice tone="danger">{error}</Notice>}
      </div>
    </Modal>
  );
}

function Workspace({ access, application, ready }: { access: IDEAccess; application: IDEApplication; ready: boolean }) {
  const { tr } = useI18n();
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState<{code:string;at:string;requestID:string}>();
  const [copied,setCopied] = useState(false);
  useEffect(() => {
    if (!ready || !access.enabled) return;
    const controller = new AbortController();
    let stage = 'connector';
    let requestID='';
    const receiveID=(id:string)=>{requestID=id;};
    const fail = (code:string) => setError({code,at:new Date().toISOString(),requestID});
    const timeout = window.setTimeout(() => {
      controller.abort();
      fail('timeout');
    }, 90000);
    setError(undefined);setCopied(false);
    void (async () => {
      // Read-only preflight is optional: runtime remains the authorization boundary.
      const devices = await ideRequest<IDEConnector[]>('connectors','GET',undefined,controller.signal,receiveID).catch(()=>undefined);
      if(controller.signal.aborted)return;
      const device = devices?.find(c=>c.id===application.edge_id);
      if(device&&!device.online){fail('offline');return;}
      stage = 'runtime';
      requestID='';
      const result = await ideRequest<IDEResult>(
        `accesses/${access.id}/runtime`, 'POST',
        { action: 'start', project: access.project || '' }, controller.signal,receiveID,
      );
      if (controller.signal.aborted) return;
      if (result.status !== 'ok' || !result.instances?.[0]) {
        fail(['non_root_required','not_found','start_failed','installing','install_failed'].includes(result.status)?result.status:'runtime');return;
      }
      stage = 'launch';
      requestID='';
      const target = await ideRequest<{ url: string }>(
        `accesses/${access.id}/launch`, 'POST',
        { instance_id: result.instances[0].id, project: access.project || '', theme: ideTheme() }, controller.signal,receiveID,
      );
      if (controller.signal.aborted) return;
      const url = new URL(target.url);
      if (url.protocol !== 'https:' || url.hostname !== window.location.hostname || url.username || url.password)
        throw Error();
      window.location.replace(url.href);
    })().catch((cause) => {
      if (!controller.signal.aborted) {
        const status = cause instanceof RequestError ? cause.response?.status : undefined;
        fail(status===403?'forbidden':status===404?'not_found':cause instanceof RequestError && !status?'network':stage);
      }
    }).finally(() => window.clearTimeout(timeout));
    return () => { window.clearTimeout(timeout); controller.abort(); };
  }, [access.id, access.project, access.enabled, application.edge_id, ready, attempt]);
  const messages:Record<string,[string,string]> = {
    offline:['连接器离线。请先恢复设备连接，再重试。','The connector is offline. Reconnect the device and retry.'],
    timeout:['打开 IDE 超时。设备任务可能仍在运行，请稍后重试。','Opening the IDE timed out. The device task may still be running; retry shortly.'],
    non_root_required:['设备账号或平台不支持启动 IDE，请检查应用安装。','The device account or platform cannot launch this IDE. Check the application installation.'],
    not_found:['所需访问或运行资源不存在，请检查应用配置。','The required access or runtime resource was not found. Check the application configuration.'],
    start_failed:['设备未能启动 IDE，请检查 code-server 安装与设备资源。','The device could not start the IDE. Check code-server and device resources.'],
    installing:['code-server 仍在安装，完成后请重试。','code-server is still installing. Retry when installation completes.'],
    install_failed:['code-server 安装失败，请在应用中重新检查安装。','code-server installation failed. Check the installation under Applications.'],
    forbidden:['没有使用权限或访问已禁用，请联系管理员。','Access is disabled or you do not have permission. Contact your administrator.'],
    network:['无法连接服务，请检查网络后重试。','Cannot reach the service. Check your network and retry.'],
    launch:['IDE 访问入口准备失败，请检查入口配置后重试。','The IDE entry could not be prepared. Check the ingress configuration and retry.'],
    runtime:['设备启动请求失败，请检查连接器与安装后重试。','The device start request failed. Check the connector and installation, then retry.'],
  };
  const diagnostic = error ? `IDE_OPEN/${error.code.toUpperCase()} · ${error.at}${error.requestID?` · request_id=${error.requestID}`:''}` : '';
  if (error || !ready || !access.enabled) return (
    <Notice tone="danger">
      {!access.enabled ? tr('此访问已禁用。', 'This access is disabled.')
        : !ready ? tr('IDE 入口尚未就绪，请联系管理员。', 'The IDE entry is not ready. Contact your administrator.')
        : tr(...(messages[error?.code||'runtime']||messages.runtime))}
      {error&&<p className="webide-diagnostic"><code>{diagnostic}</code></p>}
      <div className="webide-actions">
        {ready && access.enabled && <Button onClick={() => setAttempt(v => v + 1)}>{tr('重试', 'Retry')}</Button>}
        {error&&<Button onClick={()=>{void navigator.clipboard.writeText(diagnostic).then(()=>setCopied(true)).catch(()=>setCopied(false));}}>{copied?tr('已复制','Copied'):tr('复制诊断信息','Copy diagnostics')}</Button>}
        <Link to="/access/webide">{tr('返回访问列表', 'Back to accesses')}</Link>
      </div>
    </Notice>
  );
  return <div role="status" className="webide-actions"><span className="ui-spinner" aria-hidden />{tr('正在打开 IDE…', 'Opening IDE…')}</div>;
}
