import {useI18n} from '@/i18n';

// Plain text only: never turn model-supplied file paths or diff contents into HTML.
export function DiffView({text}:{text:string}){
 const {tr}=useI18n();let oldLine:number|undefined,newLine:number|undefined,oldRemaining=0,newRemaining=0;
 return <div className="edge-agent-diff" role="region" aria-label={tr('代码差异','Code diff')} tabIndex={0}>
  {text.split('\n').map((line,index)=>{
   const hunk=/^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/.exec(line);
   let kind='',oldNumber='',newNumber='';
   if(hunk){oldLine=Number(hunk[1]);newLine=Number(hunk[3]);oldRemaining=Number(hunk[2]??1);newRemaining=Number(hunk[4]??1);kind='hunk';}
   else if(oldRemaining===0&&newRemaining===0&&(line.startsWith('diff ')||line.startsWith('index ')||line.startsWith('---')||line.startsWith('+++'))){kind='header';oldLine=newLine=undefined;}
   else if(line.startsWith('+')){kind='added';if(newLine!==undefined)newNumber=String(newLine++);newRemaining=Math.max(0,newRemaining-1);}
   else if(line.startsWith('-')){kind='removed';if(oldLine!==undefined)oldNumber=String(oldLine++);oldRemaining=Math.max(0,oldRemaining-1);}
   else if(line.startsWith(' ')){if(oldLine!==undefined)oldNumber=String(oldLine++);if(newLine!==undefined)newNumber=String(newLine++);oldRemaining=Math.max(0,oldRemaining-1);newRemaining=Math.max(0,newRemaining-1);}
   return <div key={index} className={`edge-agent-diff-line ${kind}`}><span aria-hidden>{oldNumber}</span><span aria-hidden>{newNumber}</span><code>{line||' '}</code></div>;
  })}
 </div>;
}
