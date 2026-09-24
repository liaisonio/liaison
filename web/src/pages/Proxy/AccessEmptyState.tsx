import {ArrowRight, Bot, Cable, Database, FolderOpen, Globe2, Layers, Monitor, Search, Terminal} from 'lucide-react';
import type {ReactNode} from 'react';
import {Button} from '@/components/ui';
import {useI18n} from '@/i18n';
import './groups.less';

export function EmptyAccessLayout({icon,title,description,children,className=''}:{icon:ReactNode;title:ReactNode;description:string;children?:ReactNode;className?:string}) {
  return <div className={`liaison-access-empty ${className}`}><span className="liaison-access-empty-icon" aria-hidden>{icon}</span><div className="liaison-access-empty-title">{title}</div><p>{description}</p>{children}</div>;
}

export default function AccessEmptyState({category,onCreate,onReset,actionLabel}:{category?:string;onCreate?:()=>void;onReset?:()=>void;actionLabel?:string}) {
  const {tr}=useI18n();
  const types:Record<string,{icon:typeof Globe2;title:string;description:string}>={
    web:{icon:Globe2,title:tr('接入 Web 应用','Connect a web application'),description:tr('通过连接器连接网站，在浏览器中安全访问。','Reach your website securely through a connector.')},
    ssh:{icon:Terminal,title:tr('接入 SSH / SFTP','Connect SSH / SFTP'),description:tr('通过连接器连接服务器，使用终端或管理文件。','Connect to your server through a connector to use the terminal or manage files.')},
    database:{icon:Database,title:tr('接入数据库','Connect a database'),description:tr('通过连接器连接数据库，在浏览器中查询和管理数据。','Connect to your database through a connector to query and manage data in the browser.')},
    cache:{icon:Layers,title:tr('接入缓存服务','Connect a cache service'),description:tr('通过连接器连接缓存服务，在浏览器中查看和管理数据。','Connect to your cache service through a connector to view and manage data in the browser.')},
    storage:{icon:FolderOpen,title:tr('接入存储服务','Connect a storage service'),description:tr('通过连接器连接存储服务，在浏览器中管理文件。','Connect to your storage service through a connector to manage files in the browser.')},
    desktop:{icon:Monitor,title:tr('接入远程桌面','Connect a remote desktop'),description:tr('通过连接器连接设备，在浏览器中使用远程桌面。','Connect to your device through a connector to use its remote desktop in the browser.')},
    tcp:{icon:Cable,title:tr('接入 TCP 服务','Connect a TCP service'),description:tr('通过连接器连接 TCP 服务，创建供客户端连接的访问入口。','Reach your TCP service through a connector and create an endpoint for your clients.')},
    agent:{icon:Bot,title:tr('接入设备上的 Agent','Connect an Agent on your device'),description:tr('通过连接器使用设备上已安装的 Agent，创建和继续会话。','Use an Agent already installed on your device through a connector to start and resume conversations.')},
  };
  const content=types[category||'']||{icon:Globe2,title:tr('创建第一个访问','Create your first access'),description:tr('通过连接器连接应用或模型服务，创建安全访问入口。','Connect to an application or model service through a connector to create a secure access endpoint.')};
  const Icon=onReset?Search:content.icon;
  return <EmptyAccessLayout icon={<Icon size={20}/>} title={<h3>{onReset?tr('暂无匹配访问','No matching access'):content.title}</h3>} description={onReset?tr('试试其他筛选条件，或重置筛选。','Try different filters or reset them.'):content.description}>
    {onReset?<Button onClick={onReset}>{tr('重置筛选','Reset filters')}</Button>:onCreate&&<Button variant="primary" onClick={onCreate}>{actionLabel||tr('新建访问','Create access')}<ArrowRight size={14}/></Button>}
  </EmptyAccessLayout>;
}
