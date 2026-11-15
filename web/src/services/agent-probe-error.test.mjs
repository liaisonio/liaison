import assert from 'node:assert/strict';
import test from 'node:test';
import {agentEdgeID} from './agent-edge-id.ts';
import {agentProbeError,AgentProbeStatusError} from './agent-probe-error.ts';

test('connector IDs preserve uint64 strings and reject invalid values',()=>{
  for(const id of [1,'1','9007199254740993','18446744073709551615'])assert.equal(agentEdgeID(id),id);
  for(const id of [0,-1,1.5,NaN,Number.MAX_SAFE_INTEGER+1,'','0','01','-1','1e2','1.0','18446744073709551616',null,undefined])assert.throws(()=>agentEdgeID(id),/INVALID_AGENT_CONNECTOR/);
});
test('HTTP and connector errors are localized and never expose raw errors',()=>{
  const cases=[[400,'未接受','server rejected'],[401,'登录已失效','session expired'],[403,'权限','cannot discover'],[404,'不可用','unavailable'],[429,'频繁','Too many'],[504,'超时','timed out']];
  for(const [status,zh,en] of cases){
    const err={response:{status},message:'SECRET'};
    assert.ok(agentProbeError(err,(a)=>a).includes(zh));
    assert.ok(agentProbeError(err,(_,b)=>b).includes(en));
  }
  for(const [status,expected] of [['upgrade_required','升级'],['unsupported_user','普通用户'],['unavailable','在线']])assert.ok(agentProbeError(new AgentProbeStatusError(status),a=>a).includes(expected));
  assert.match(agentProbeError(new Error('INVALID_AGENT_CONNECTOR'),a=>a),/标识无效/);
  assert.match(agentProbeError(Object.assign(new Error('secret'),{name:'TimeoutError'}),a=>a),/超时/);
  for(const err of [null,undefined,new Error('SECRET'),new AgentProbeStatusError('SECRET')])assert.equal(agentProbeError(err,a=>a),'Agent 探测未完成，请稍后重试。');
});
