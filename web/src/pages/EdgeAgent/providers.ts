export const agentKinds = [{value:'codex',label:'Codex'},{value:'claude',label:'Claude Code'}];
export const agentLabel = (kind:string) => agentKinds.find(item=>item.value===kind)?.label || kind || 'Agent';
