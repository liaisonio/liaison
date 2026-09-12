import {useI18n} from '@/i18n';
import {hasCommandRisk} from './commandRisk';
import type {TerminalCompletionView} from './terminalCompletion';

export default function CommandCompletion({view, onAccept}: {view: TerminalCompletionView; onAccept: (candidate: string) => void}) {
  const {tr} = useI18n();
  const risky = hasCommandRisk(view.candidates[view.selected] || '');
  return <div className={`webssh-completion${view.above ? ' is-above' : ''}${risky ? ' is-risky' : ''}`}
    role="listbox" aria-label={tr('命令补全', 'Command completion')}
    style={{left:view.left+14, top:view.top+12}}
    onMouseDown={event=>{event.preventDefault();event.stopPropagation();}}
    onClick={event=>event.stopPropagation()}>
    {view.candidates.map((candidate,index)=><button key={candidate} type="button" role="option" tabIndex={-1} title={candidate}
      aria-selected={index===view.selected} className={index===view.selected?'is-selected':''} onClick={()=>onAccept(candidate)}>
      <span><span className="typed">{view.input}</span>{candidate.slice(view.input.length)}</span>
      {index===view.selected&&<kbd>Tab</kbd>}
    </button>)}
    {risky&&<small className="command-risk" role="status">{tr('⚠ 危险操作，请核对目标和参数', '⚠ Risky operation: review target and arguments')}</small>}
    <small>{tr('AI 建议 · Tab 填入，不执行', 'AI suggestion · Tab inserts, never executes')}</small>
  </div>;
}
