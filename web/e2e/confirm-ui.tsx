import React from 'react';
import {createRoot} from 'react-dom/client';
import {Button,Modal,DangerConfirm} from '../src/components/ui';
import {useI18n} from '../src/i18n';
import {applyThemeOnBoot} from '../src/store/theme';
import '../src/styles/index.css';
if(!import.meta.env.DEV)throw Error('Development fixture only');
applyThemeOnBoot();
function Demo(){const {tr}=useI18n();const [open,setOpen]=React.useState(true);return <Modal open={open} width={450} title={tr('删除连接器','Delete connector')} onClose={()=>setOpen(false)} footer={<><Button onClick={()=>setOpen(false)}>{tr('取消','Cancel')}</Button><Button variant="danger">{tr('删除','Delete')}</Button></>}><DangerConfirm title={tr('删除“Connector-example”？','Delete “Connector-example”?')} description={tr('承载的应用、访问和密钥关系将一并移除，历史记录仍会保留。','Applications, access entries and key relations will be removed. History is retained.')}/></Modal>}
createRoot(document.getElementById('root')!).render(<Demo/>);
