import { useLayoutEffect, useState, type RefObject } from 'react';
const compactQuery = '(max-width: 850px), (pointer: coarse) and (max-height: 600px)';
export function useMobileViewport(workspace: RefObject<HTMLDivElement>, active: boolean) {
  const [compact, setCompact] = useState(() => matchMedia(compactQuery).matches);
  useLayoutEffect(() => {
    const media = matchMedia(compactQuery);
    const node = workspace.current;
    let frame = 0;
    const update = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        setCompact(media.matches);
        const viewport = window.visualViewport;
        const nativeHeight = Number.parseFloat(document.documentElement.style.getPropertyValue('--ssh-native-height'));
        const height = Number.isFinite(nativeHeight) && nativeHeight > 0 ? nativeHeight : viewport?.height ?? innerHeight;
        const offset = Number.isFinite(nativeHeight) && nativeHeight > 0 ? 0 : viewport?.offsetTop ?? 0;
        document.documentElement.style.setProperty('--ssh-viewport-height', `${height}px`);
        document.documentElement.style.setProperty('--ssh-viewport-top', `${offset}px`);
        if (!node) return;
        if (!media.matches) { node.style.removeProperty('--terminal-mobile-height'); return; }
        const bottom = height + offset;
        const bottomGap = document.documentElement.hasAttribute('data-liaison-embedded') ? 0 : 8;
        node.style.setProperty('--terminal-mobile-height', `${Math.max(100, bottom - node.getBoundingClientRect().top - bottomGap)}px`);
      });
    };
    update();
    media.addEventListener('change', update);
    window.addEventListener('resize', update);
    window.addEventListener('liaison:viewport', update);
    window.visualViewport?.addEventListener('resize', update);
    window.visualViewport?.addEventListener('scroll', update);
    const parent = node?.parentElement;
    parent?.addEventListener('scroll', update, { passive: true });
    return () => {
      cancelAnimationFrame(frame); media.removeEventListener('change', update);
      window.removeEventListener('resize', update);
      window.removeEventListener('liaison:viewport', update);
      window.visualViewport?.removeEventListener('resize', update);
      window.visualViewport?.removeEventListener('scroll', update);
      parent?.removeEventListener('scroll', update);
      node?.style.removeProperty('--terminal-mobile-height');
    };
  }, [workspace, active]);
  return compact;
}
