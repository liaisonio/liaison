import {useEffect,useRef,useState} from 'react';
import {useI18n} from '@/i18n';

export default function TokenTrend({points,bucketMs=86400000}:{points:[string,number][];bucketMs?:number}){
 const {tr}=useI18n();const [hover,setHover]=useState<string>();
 const container=useRef<HTMLDivElement>(null);const [width,setWidth]=useState(640);
 useEffect(()=>{if(!container.current)return;const observer=new ResizeObserver(([entry])=>setWidth(Math.max(260,Math.round(entry.contentRect.width))));observer.observe(container.current);return()=>observer.disconnect();},[points.length>0]);
 const right=width-24;
 const selected=points.find(([date])=>date===hover)||points.at(-1);
 if(!selected)return <p>{tr('暂无已确认用量','No confirmed usage yet')}</p>;
 const peak=Math.max(4,...points.map(([,value])=>value));
 const start=Date.parse(points[0][0]),end=Date.parse(points[points.length-1][0]);
 const coords=points.map(([date,value])=>({date,value,x:end===start?(60+right)/2:60+(Date.parse(date)-start)/(end-start)*(right-60),y:170-value/peak*140}));
 const segments:typeof coords[]=[];
 for(const point of coords){const last=segments.at(-1);if(!last||Date.parse(point.date)-Date.parse(last.at(-1)!.date)>bucketMs*1.01)segments.push([point]);else last.push(point);}
 const curve=(segment:typeof coords)=>segment.map((p,i)=>{if(!i)return `M ${p.x} ${p.y}`;const prev=segment[i-1],mid=(prev.x+p.x)/2;return `C ${mid} ${prev.y}, ${mid} ${p.y}, ${p.x} ${p.y}`;}).join(' ');
 const active=coords.find(p=>p.date===selected[0])!;
 return <div className="ai-token-trend">
  <div className="ai-token-trend-legend"><span>{tr('已报告 Token','Reported tokens')}</span><span><time>{selected[0].slice(0,16).replace('T',' ')} UTC</time><strong>{selected[1].toLocaleString()}</strong></span></div>
  <div className="ai-token-trend-scroll" ref={container}><svg viewBox={`0 0 ${width} 210`} role="group" tabIndex={0} aria-label={tr('Token 用量趋势，按 UTC 时间。左右方向键查看数据。','Token usage trend by UTC time. Use arrow keys to inspect points.')} onPointerMove={event=>{const rect=event.currentTarget.getBoundingClientRect();const x=(event.clientX-rect.left)/rect.width*width;setHover(coords.reduce((a,b)=>Math.abs(a.x-x)<Math.abs(b.x-x)?a:b).date);}} onPointerLeave={()=>setHover(undefined)} onKeyDown={event=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;event.preventDefault();const index=coords.indexOf(active);setHover(coords[event.key==='Home'?0:event.key==='End'?coords.length-1:Math.max(0,Math.min(coords.length-1,index+(event.key==='ArrowLeft'?-1:1)))].date);}}>
   {[0,0.25,0.5,0.75,1].map(r=><g key={r}><line className="ai-token-grid" x1="60" x2={right} y1={170-r*140} y2={170-r*140}/><text x="48" y={174-r*140} textAnchor="end">{Intl.NumberFormat(undefined,{notation:'compact',maximumFractionDigits:1}).format(peak*r)}</text></g>)}
   {segments.filter(s=>s.length>1).map(segment=><g key={segment[0].date}><path className="ai-token-area" d={`${curve(segment)} L ${segment.at(-1)!.x} 170 L ${segment[0].x} 170 Z`}/><path className="ai-token-line" d={curve(segment)}/></g>)}
   <line className="ai-token-grid" x1={active.x} x2={active.x} y1="30" y2="170"/>
   {coords.map(p=><circle key={p.date} className="ai-token-point" cx={p.x} cy={p.y} r={p.date===selected[0]?4.5:2} role="img" aria-label={`${p.date}: ${p.value} Token`}><title>{p.date}: {p.value.toLocaleString()} Token</title></circle>)}
   {[...new Set(width<450?[0,coords.length-1]:[0,Math.floor((coords.length-1)/2),coords.length-1])].map(i=><text key={i} x={coords[i].x} y="198" textAnchor={coords.length===1?'middle':i===0?'start':i===coords.length-1?'end':'middle'}>{end-start>=86400000?coords[i].date.slice(5,10):coords[i].date.slice(11,16)}</text>)}
  </svg></div>
  {points.length===1&&<p>{tr('当前只有一个时间点的数据，更多时间点产生用量后会形成折线。','Only one time point is available. A line appears as more time points report usage.')}</p>}
 </div>;
}
