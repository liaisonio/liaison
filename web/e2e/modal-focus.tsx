import React, {useRef, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Button, Input, Modal} from '../src/components/ui';
import '../src/styles/index.css';
if (!import.meta.env.DEV) throw Error('Development fixture only');
function Demo() {
 const host=useRef<HTMLDivElement>(null);
 const [open,setOpen]=useState(false),[nested,setNested]=useState(false);
 return <div ref={host} data-focus-host>
  <Button onClick={()=>void host.current?.requestFullscreen()}>Fullscreen</Button>
  <Button onClick={()=>setOpen(true)}>Open dialog</Button>
  <Modal open={open} title="Focus test" onClose={()=>setOpen(false)} footer={<Button onClick={()=>setNested(true)}>Nested dialog</Button>}>
   <Input aria-label="Project name"/>
   <Modal open={nested} title="Nested test" onClose={()=>setNested(false)}><Input aria-label="Nested name"/></Modal>
  </Modal>
 </div>;
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><Demo/></React.StrictMode>);
