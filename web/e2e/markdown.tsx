import React from 'react';
import {createRoot} from 'react-dom/client';
import {MessageContent} from '../src/components/AgentWorkspace/MessageContent';
import '../src/styles/index.css';
import '../src/components/AgentWorkspace/index.less';
const text = '# 连接检查\n\n## 当前状态\n\n**正常**，支持 *强调* 与 `inline`。\n\n- 连接器\n  - 在线\n  - 已认证\n\n1. 查看设备\n2. 查看应用\n\n> 只读检查，不修改配置。\n\n| 应用 | 状态 |\n| --- | --- |\n| MySQL | 正常 |\n\n```sql\nSELECT 1;\n```\n\n[官网](https://liaison.cloud) [unsafe](javascript:alert(1))\n\n<img src="https://example.com/tracker" onerror="alert(1)">\n\n![tracking](https://example.com/pixel)';
createRoot(document.getElementById('root')!).render(<main style={{maxWidth:700,padding:24,margin:'auto'}}><MessageContent text={text}/><section id="stream"><MessageContent text={'```sql\nSELECT'}/></section></main>);
