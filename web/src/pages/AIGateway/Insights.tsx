import TokenTrend from './TokenTrend';
import {useEffect,useState} from 'react';
import {request} from '@/api/client';
import {Button,Drawer,Notice,StatusPill} from '@/components/ui';
import {useI18n} from '@/i18n';

export type RequestRecord={request_id:string;key_id:number;model:string;status:number;duration_ms:number;input_tokens?:number;output_tokens?:number;complete:boolean};
type Usage={summary:{requests:number;unknown_requests:number;input_tokens?:number;output_tokens?:number};records:{created_at:string;input_tokens?:number;output_tokens?:number}[]};
export function Insights({base,records,models}:{base:string;records:RequestRecord[];models:string[]}){
 const {tr}=useI18n();const [hours,setHours]=useState(720);const [usage,setUsage]=useState<Usage>();const [failed,setFailed]=useState(false);const [revision,setRevision]=useState(0);
 const [rangeEnd,setRangeEnd]=useState(Date.now);
 useEffect(()=>{let alive=true;setUsage(undefined);setFailed(false);request<API.Response<Usage>>(`${base}/usage?hours=${hours}`).then(r=>{if(r.code!==200||!r.data?.summary)throw Error();if(alive)setUsage(r.data);}).catch(()=>{if(alive)setFailed(true);});return()=>{alive=false;};},[base,revision,hours]);
 const success=records.filter(r=>r.status>=200&&r.status<300&&r.complete);
 const bucketMinutes=hours===1?1:hours===6?5:hours===24?30:hours===168?360:1440;
 const bucketMs=bucketMinutes*60000;
 const days=new Map<string,number>();for(const r of usage?.records||[]){if(r.input_tokens==null||r.output_tokens==null)continue;const timestamp=Date.parse(r.created_at);if(!Number.isFinite(timestamp))continue;const bucket=new Date(Math.floor(timestamp/bucketMs)*bucketMs).toISOString();days.set(bucket,(days.get(bucket)||0)+r.input_tokens+r.output_tokens);}
 const rangeLabel=hours<24?tr(`${hours} 小时`,`${hours}h`):hours===24?tr('24 小时','24h'):tr(`${hours/24} 天`,`${hours/24} days`);
 const confirmedZero=!!usage&&(usage.summary.requests===0||usage.summary.unknown_requests===0&&usage.summary.input_tokens===0&&usage.summary.output_tokens===0);
 useEffect(()=>{if(usage)setRangeEnd(Date.now());},[usage]);
 const points: [string,number][]=confirmedZero?Array.from({length:Math.round(hours*3600000/bucketMs)+1},(_,i)=>[new Date(Math.floor(rangeEnd/bucketMs)*bucketMs-hours*3600000+i*bucketMs).toISOString(),0]):[...days].sort(([a],[b])=>a.localeCompare(b));
 return <section className="ai-api-card ai-insights">
  <div className="ai-insights-heading"><h2>{tr('我的调用概览','My usage overview')}</h2><Button onClick={()=>setRevision(n=>n+1)}>{tr('刷新用量','Refresh usage')}</Button></div>
  <div className="ai-usage-ranges" role="group" aria-label={tr('用量时间范围','Usage time range')}>{[1,6,24,168,720].map(h=><Button key={h} variant={hours===h?'primary':'secondary'} aria-pressed={hours===h} onClick={()=>setHours(h)}>{h<=24?tr(`${h} 小时`,`${h}h`):tr(`${h/24} 天`,`${h/24} days`)}</Button>)}</div>
  <p>{tr('仅统计当前用户。时间筛选影响请求量、Token 总量和趋势；成功率与耗时仍基于最近 50 条请求。','Current user only. The range filters request counts, token totals and trends; success and duration still cover the latest 50 requests.')}</p>
  {failed&&<Notice tone="danger">{tr('用量加载失败，请重试。','Could not load usage. Please retry.')}</Notice>}
  <div className="ai-insights-metrics">
   {[[tr(`请求量 · ${rangeLabel}`,`Requests · ${rangeLabel}`),usage?.summary.requests?.toLocaleString()],[tr(`已报告 Token · ${rangeLabel}`,`Reported tokens · ${rangeLabel}`),confirmedZero?'0':usage?.summary.input_tokens!=null&&usage.summary.output_tokens!=null?(usage.summary.input_tokens+usage.summary.output_tokens).toLocaleString():undefined],[tr('成功率 · 最近请求','Success · recent requests'),records.length?`${(success.length/records.length*100).toFixed(1)}%`:'—'],[tr('平均耗时 · 成功请求','Mean duration · successful requests'),success.length?`${Math.round(success.reduce((v,r)=>v+r.duration_ms,0)/success.length)} ms`:'—']].map(([label,value])=><div key={label}><span>{label}</span><strong>{value??(failed?'—':usage?tr('未报告','Not reported'):tr('加载中…','Loading…'))}</strong></div>)}
  </div>
  {!!usage?.summary.unknown_requests&&<p>{tr(`${usage.summary.unknown_requests} 次请求用量未确认，不计为零。`,`${usage.summary.unknown_requests} requests have unconfirmed usage; not counted as zero.`)}</p>}
  <h2>{tr('Token 用量趋势','Token usage trend')}</h2><p>{confirmedZero?tr('所选时间范围内已确认用量为 0 · UTC。','Confirmed zero usage in the selected time range · UTC.'):tr(`最近 100 条用量记录 · ${bucketMinutes<60?`${bucketMinutes} 分钟`:`${bucketMinutes/60} 小时`}粒度 · UTC。空档不补零，非完整趋势。`,`Latest 100 usage records · ${bucketMinutes<60?`${bucketMinutes} min`:`${bucketMinutes/60}h`} buckets · UTC. Gaps are not zero-filled; partial trend.`)}</p>
  {!usage&&!failed?<p role="status">{tr('加载中…','Loading…')}</p>:usage?<TokenTrend key={hours} points={points} bucketMs={bucketMs} rangeStart={rangeEnd-hours*3600000} rangeEnd={rangeEnd}/>:null}
  <h2>{tr('模型最近调用','Recent model results')}</h2><p>{tr('根据当前用户最近请求展示，不代表主动健康探测。','Based on your recent requests, not active health probes.')}</p>
  <div className="ai-insights-models">{models.map(model=>{const last=records.find(r=>r.model===model);return <div key={model}><code>{model}</code><StatusPill tone={!last?'neutral':last.status<300&&last.complete?'success':'danger'}>{!last?tr('尚未调用','Not called'):last.status<300&&last.complete?tr('最近成功','Last call succeeded'):tr('最近未完成','Last call failed or incomplete')}</StatusPill></div>;})}</div>
 </section>;
}
export function RequestDetails({record,onClose}:{record?:RequestRecord;onClose:()=>void}){
 const {tr}=useI18n();const [copied,setCopied]=useState(0);useEffect(()=>setCopied(0),[record]);
 useEffect(()=>{if(!copied)return;const timer=window.setTimeout(()=>setCopied(0),2500);return()=>window.clearTimeout(timer);},[copied]);
 return <Drawer open={!!record} title={tr('请求详情','Request details')} onClose={onClose}>{record&&<>
 <dl className="liaison-detail-list">{[[tr('请求 ID','Request ID'),record.request_id],[tr('模型','Model'),record.model],[tr('调用来源','Source'),record.key_id?`${tr('密钥','Key')} #${record.key_id}`:tr('在线体验','Playground')],[tr('HTTP 状态','HTTP status'),record.status],[tr('完成状态','Completion'),record.complete?tr('已完成','Complete'):tr('未完成','Incomplete')],[tr('总耗时','Total duration'),`${record.duration_ms} ms`],[tr('输入 Token','Input tokens'),record.input_tokens??tr('未报告','Not reported')],[tr('输出 Token','Output tokens'),record.output_tokens??tr('未报告','Not reported')],[tr('首字延迟','First token latency'),tr('未采集','Not collected')],[tr('协议转换与失败环节','Protocol conversion and failure stage'),tr('历史记录未采集','Not captured in historical records')]].map(([label,value])=><div key={label}><dt>{label}</dt><dd style={{overflowWrap:'anywhere'}}>{value}</dd></div>)}</dl>
 <Button onClick={()=>void navigator.clipboard.writeText(record.request_id).then(()=>setCopied(Date.now())).catch(()=>setCopied(0))}>{copied?tr('已复制','Copied'):tr('复制请求 ID','Copy request ID')}</Button>
 <p>{tr('不记录提示词、回答或密钥原文。','Prompts, answers and key secrets are not logged.')}</p>
 </>}</Drawer>;
}
