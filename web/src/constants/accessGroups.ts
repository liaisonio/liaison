import {LLM_PROTOCOL_OPTIONS,protocolFamily} from './llmProtocols';
import {ACCESS_TYPES} from './accessTypes';
import {optionalProtocolEnabled} from '../store/optionalProtocols';

// Business categories, not transport layers. Only implemented access types belong here.
export const ACCESS_GROUPS = [
  {value:'web',label:'Web',types:['http']},
  {value:'llm',label:'LLM',types:[...LLM_PROTOCOL_OPTIONS.map(p=>p.value),'aiapi']},
  {value:'ssh',label:'SSH/SFTP',types:['webssh','websftp','ssh']},
  {value:'database',label:'Database',types:['webmysql','webmariadb','webdoris','webstarrocks','webtidb','webpostgresql','websqlserver','weboracle','webdameng','webmongodb','webclickhouse','webelasticsearch','webopensearch']},
  {value:'cache',label:'Cache',types:['webredis','webmemcached']},
  {value:'desktop',label:'Desktop',types:['webrdp','webvnc']},
  {value:'storage',label:'Storage',types:['webs3','websmb']},
  {value:'tcp',label:'TCP',types:['tcp']},
];
export function accessGroup(search:URLSearchParams) {
  // Preserve bookmarks made before Redis moved from Database to Cache.
  if(search.get('category')==='database' && search.get('access_type')==='webredis') {
    return ACCESS_GROUPS.find(group=>group.value==='cache');
  }
  const requested=ACCESS_GROUPS.find(group=>group.value===search.get('category'));
  return requested || ACCESS_GROUPS.find(group=>group.types.includes(protocolFamily(search.get('access_type'))||''));
}
export function groupTypes(group:typeof ACCESS_GROUPS[number]) {
  return group.types.flatMap(type=>ACCESS_TYPES.filter(item=>item.value===type&&optionalProtocolEnabled(type)));
}
export function accessTabLabel(label:string){return label.startsWith('Web ')?label.replace('Web ','Web'):label;}
