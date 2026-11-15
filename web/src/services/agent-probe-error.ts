type Translate=(zh:string,en:string)=>string;
export class AgentProbeStatusError extends Error {
  readonly status:string;
  constructor(status:string){super('Agent discovery unavailable');this.status=status;}
}
export function agentProbeError(error:unknown,tr:Translate):string {
  const code=(error as {response?:{status?:number}})?.response?.status;
  if(error instanceof Error&&error.message==='INVALID_AGENT_CONNECTOR')return tr('连接器标识无效，请关闭弹窗并重新选择连接器。','Invalid connector identifier. Close this dialog and select the connector again.');
  if(code===400)return tr('服务端未接受本次探测请求。重试后仍失败，请检查 Manager 与连接器的版本兼容性。','The server rejected the discovery request. If retrying fails, check Manager and connector version compatibility.');
  if(code===401)return tr('登录已失效，请重新登录后探测。','Your session expired. Sign in again to discover Agents.');
  if(code===403)return tr('当前账号没有此连接器的 Agent 探测权限。','Your account cannot discover Agents on this connector.');
  if(code===404)return tr('连接器或关联资源已不可用，请刷新列表。','The connector or associated resource is unavailable. Refresh the list.');
  if(code===429)return tr('探测请求过于频繁，请稍后重试。','Too many discovery requests. Retry shortly.');
  if(code===504||(error instanceof Error&&error.name==='TimeoutError'))return tr('Agent 探测超时，请稍后重试。','Agent discovery timed out. Please retry.');
  if(error instanceof AgentProbeStatusError){
    if(error.status==='upgrade_required')return tr('连接器尚不支持 Agent 探测，请升级连接器。','This connector does not support Agent discovery. Upgrade the connector.');
    if(error.status==='unsupported_user')return tr('连接器运行账号不支持 Agent 探测，请使用普通用户运行。','Agent discovery is not supported for this connector account. Run it as a regular user.');
    if(error.status==='unavailable')return tr('连接器暂时无法完成 Agent 探测，请确认在线后重试。','The connector cannot complete Agent discovery right now. Check connectivity and retry.');
  }
  return tr('Agent 探测未完成，请稍后重试。','Agent discovery could not complete. Please retry.');
}
