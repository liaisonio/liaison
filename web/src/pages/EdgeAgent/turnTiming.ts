import type {AgentSnapshot} from '@/services/edgeAgent';

export type TurnTiming={requestMs?:number;serviceMs?:number;firstReplyMs?:number;finishedMs?:number;state:'waiting'|'running'|'completed'|'unconfirmed';background:boolean};

// All timestamps come from the same browser's monotonic clock. These are not
// provider latency metrics and must never be labelled as network or model time.
export class TurnTimingTracker {
  readonly value:TurnTiming={state:'waiting',background:false};
  private readonly baseline:string[];
  constructor(private readonly baselineSession:Pick<AgentSnapshot,'session_id'|'window'|'messages'>,private readonly started:number){
    this.baseline=(baselineSession.messages||[]).filter(m=>m.role==='assistant').map(m=>m.text);
  }
  requestFinished(now:number,confirmed:boolean,serviceMs?:number){
    this.value.requestMs=Math.max(0,now-this.started);
    if(confirmed&&serviceMs!==undefined&&Number.isFinite(serviceMs)&&serviceMs>=0)this.value.serviceMs=serviceMs;
    this.value.state=confirmed?'running':'unconfirmed';
  }
  observe(snapshot:AgentSnapshot,now:number){
    if(snapshot.session_id!==this.baselineSession.session_id||this.value.requestMs===undefined||this.value.state==='unconfirmed'||this.value.finishedMs!==undefined)return;
    const elapsed=Math.max(0,now-this.started);
    const answers=(snapshot.messages||[]).filter(m=>m.role==='assistant').map(m=>m.text);
    const changedWindow=(snapshot.window||0)!==(this.baselineSession.window||0);
    if(this.value.firstReplyMs===undefined&&answers.some((text,i)=>text.length>0&&(changedWindow||text!==this.baseline[i])))this.value.firstReplyMs=elapsed;
    if(!snapshot.running){this.value.finishedMs=elapsed;this.value.state='completed';}
  }
}
