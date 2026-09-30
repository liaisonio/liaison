import { common, createLowlight } from 'lowlight';
import { useMemo, type ReactNode } from 'react';

const highlighter = createLowlight(common);
type Node = { type: string; value?: string; properties?: { className?: unknown }; children?: Node[] };
export default function HighlightedCode({text,language}:{text:string;language:string}) {
  const content=useMemo(()=>{
    // Only explicit, supported languages are highlighted; never render model HTML.
    if(text.length>50_000||!highlighter.registered(language.toLowerCase()))return text;
    const render=(node:Node,key:number):ReactNode=>node.type==='text'?node.value:
      <span key={key} className={Array.isArray(node.properties?.className)?node.properties.className.join(' '):undefined}>{node.children?.map(render)}</span>;
    try{return render(highlighter.highlight(language.toLowerCase(),text),0);}catch{return text;}
  },[text,language]);
  return <>{content}</>;
}
