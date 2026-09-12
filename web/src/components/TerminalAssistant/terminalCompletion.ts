import type { Terminal, IMarker } from '@xterm/xterm';
import { validDraft, validInsertion, PromptBoundary } from './completionModel';

export interface TerminalCompletionView {
  input: string;
  candidates: string[];
  selected: number;
  left: number;
  top: number;
  above: boolean;
}

// Parser is attached when xterm is created, before the first prompt arrives.
export function attachTerminalCompletion(
  terminal: Terminal,
  publish: (view: TerminalCompletionView | null) => void,
  send: (text: string) => void,
  suggest: (input: string, revision: number, signal: AbortSignal) => Promise<string>,
  status: (value: 'idle' | 'busy' | 'empty' | 'error' | 'unavailable') => void = () => {},
  commandFinished: (exitCode: number) => void = () => {},
) {
  const boundary = new PromptBoundary();
  let marker: IMarker | undefined;
  let promptPrefix = '';
  let view: TerminalCompletionView | null = null;
  let dismissed = '';
  let composing = false;
  let navigationPending = false;
  let expectedEcho: string | undefined;
  let automatic = false;
  let revision = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let abort: AbortController | undefined;
  let scheduled = '';
  const hide = () => {
    revision++; clearTimeout(timer); abort?.abort(); abort = undefined;
    scheduled = ''; view = null; publish(null); status('idle');
  };
  const suspend = () => { navigationPending = false; boundary.suspend(); hide(); };
  const readBuffer = (allowBlocked = false) => {
    const buffer = terminal.buffer.active;
    if (!boundary.editing || (boundary.blocked && !allowBlocked) || composing || terminal.hasSelection()
      || document.activeElement !== terminal.textarea
      || buffer.type !== 'normal' || !marker || marker.isDisposed
      || buffer.baseY + buffer.cursorY !== marker.line
      || buffer.viewportY !== buffer.baseY || buffer.cursorX < boundary.column) return undefined;
    const line = buffer.getLine(marker.line);
    if (!line || line.isWrapped || line.translateToString(false, 0, boundary.column) !== promptPrefix
      || line.translateToString(true, buffer.cursorX).length) return undefined;
    return line.translateToString(false, boundary.column, buffer.cursorX);
  };
  const read = () => {
    const input = readBuffer() ?? '';
    return expectedEcho !== undefined && input !== expectedEcho ? '' : input;
  };
  const render = (input: string, suffix: string) => {
    const candidates = [input + suffix];
    const screen = terminal.element?.querySelector('.xterm-screen') as HTMLElement | null;
    if (!candidates.length || !screen || !terminal.element) { hide(); return; }
    const rect = screen.getBoundingClientRect();
    const host = terminal.element.parentElement!.getBoundingClientRect();
    const rowHeight = rect.height / terminal.rows;
    const cursorY = terminal.buffer.active.cursorY;
    const selected = view?.input === input ? Math.min(view.selected, candidates.length - 1) : 0;
    const above = cursorY + candidates.length + 2 >= terminal.rows;
    view = {
      input, candidates, selected, above,
      left: Math.max(0, Math.min(rect.left - host.left + terminal.buffer.active.cursorX * rect.width / terminal.cols, host.width - 260)),
      top: rect.top - host.top + (cursorY + (above ? 0 : 1)) * rowHeight,
    };
    publish({ ...view });
  };
  const requestSuggestion = (manual = false) => {
    const input = read();
    if (expectedEcho !== undefined && input === expectedEcho) expectedEcho = undefined;
    if (!validDraft(input) || (!manual && (!automatic || input === dismissed))) {
      if (scheduled || view) hide();
      if (manual) status('unavailable');
      return;
    }
    if (!manual && scheduled === input) return;
    hide(); scheduled = input;
    const current = revision;
    timer = setTimeout(async () => {
      if (current !== revision || read() !== input) return;
      const controller = new AbortController(); abort = controller;
      const timeout = setTimeout(() => { controller.abort(); if (current === revision) status('error'); }, 15000);
      status('busy');
      try {
        const suffix = await suggest(input, current, controller.signal);
        if (controller.signal.aborted || current !== revision || read() !== input) return;
        if (!suffix) { status('empty'); return; }
        if (!validInsertion(suffix)) throw new Error('Invalid insertion');
        render(input, suffix); status('idle');
      } catch {
        if (current === revision) status('error');
      } finally { clearTimeout(timeout); }
    }, manual ? 0 : 300);
  };
  const refresh = () => requestSuggestion();
  const accept = (candidate?: string) => {
    if (!view || read() !== view.input) { hide(); return false; }
    const selected = candidate ?? view.candidates[view.selected];
    const suffix = view.candidates.includes(selected) && selected.startsWith(view.input) ? selected.slice(view.input.length) : '';
    const acceptedInput = view.input;
    hide();
    if (!validInsertion(suffix)) return false;
    expectedEcho = acceptedInput + suffix;
    dismissed = expectedEcho;
    send(suffix); // No newline, shell control sequence or replacement of typed text.
    terminal.focus();
    return true;
  };
  const parser = terminal.parser.registerOscHandler(633, data => {
    // Metadata from older integrations is never used as a candidate source.
    if (!['A', 'B', 'C', 'D'].includes(data) && !data.startsWith('D;')) return true;
    const buffer = terminal.buffer.active;
    boundary.sequence(data, buffer.baseY + buffer.cursorY, buffer.cursorX);
    if (/^D;\d+$/.test(data)) {
      const exitCode = Number(data.slice(2));
      if (exitCode <= 255) commandFinished(exitCode);
    }
    if (data === 'B') {
      marker?.dispose();
      marker = terminal.registerMarker(0);
      promptPrefix = buffer.getLine(buffer.baseY + buffer.cursorY)?.translateToString(false, 0, buffer.cursorX) || '';
      dismissed = '';
      expectedEcho = undefined;
      navigationPending = false;
    }
    hide();
    return true;
  });
  terminal.attachCustomKeyEventHandler(event => {
    if (event.type !== 'keydown') return true;
    if (['Control', 'Alt', 'Meta', 'Shift'].includes(event.key)) return true;
    if (event.isComposing || event.key === 'Process') { suspend(); return true; }
    if (event.ctrlKey && event.code === 'Space') {
      event.preventDefault(); requestSuggestion(true); return false;
    }
    if (view && !event.ctrlKey && !event.altKey && !event.metaKey) {
      if (event.key === 'Tab' && !event.shiftKey) {
        if (accept()) { event.preventDefault(); return false; }
      }
      if (event.key === 'Escape') {
        dismissed = view.input; hide(); event.preventDefault(); return false;
      }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        view.selected = (view.selected + (event.key === 'ArrowDown' ? 1 : -1) + view.candidates.length) % view.candidates.length;
        publish({ ...view }); event.preventDefault(); return false;
      }
    }
    if (event.key === 'Enter' || event.ctrlKey || event.altKey || event.metaKey
      || ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'Delete', 'Tab', 'Escape'].includes(event.key)) suspend();
    else hide(); // Wait for remote echo, not the local key event.
    return true;
  });
  const data = terminal.onData(value => {
    // Recover only after remote echo proves we are back at the end of the same
    // single-line prompt. Do not guess text from cursor/history key sequences.
    if (boundary.editing && /^(?:\x1b\[[ABCDHF]|\x1bO[HF]|\x1b\[3~|\x15|\x17)$/.test(value)) {
      boundary.suspend(); navigationPending = true; expectedEcho = undefined; hide(); return;
    }
    if (value.length !== 1 || !/^[\x20-\x7e\x7f]$/.test(value)) { suspend(); return; }
    const current = expectedEcho ?? readBuffer() ?? '';
    expectedEcho = value === '\x7f' ? current.slice(0, -1) : current + value;
    hide();
  });
  const parsed = terminal.onWriteParsed(() => {
    if (navigationPending && readBuffer(true) !== undefined) {
      navigationPending = false; boundary.blocked = false;
    }
    refresh();
  });
  // Layout/focus changes invalidate coordinates, not the shell editing phase.
  // Reflow is accepted only if the original prompt prefix and single-line marker
  // still match. Wrapped/reflowed prompts remain ineligible until a new prompt.
  const refreshLayout = () => { hide(); queueMicrotask(refresh); };
  const scroll = terminal.onScroll(refreshLayout);
  const resize = terminal.onResize(refreshLayout);
  const selection = terminal.onSelectionChange(hide);
  const onCompositionStart = () => { composing = true; suspend(); };
  const onCompositionEnd = () => { composing = false; };
  terminal.element?.addEventListener('paste', suspend, true);
  terminal.element?.addEventListener('compositionstart', onCompositionStart, true);
  terminal.element?.addEventListener('compositionend', onCompositionEnd, true);
  terminal.textarea?.addEventListener('blur', hide);
  terminal.textarea?.addEventListener('focus', refresh);
  return {
    accept,
    suggest: () => requestSuggestion(true),
    setAutomatic(enabled: boolean) { automatic = enabled; hide(); if (enabled) refresh(); },
    reset() { automatic = false; hide(); },
    dispose() {
      parser.dispose(); data.dispose(); parsed.dispose(); scroll.dispose(); resize.dispose(); selection.dispose(); marker?.dispose();
      terminal.element?.removeEventListener('paste', suspend, true);
      terminal.element?.removeEventListener('compositionstart', onCompositionStart, true);
      terminal.element?.removeEventListener('compositionend', onCompositionEnd, true);
      terminal.textarea?.removeEventListener('blur', hide);
      terminal.textarea?.removeEventListener('focus', refresh);
      terminal.attachCustomKeyEventHandler(() => true);
      hide();
    },
  };
}
