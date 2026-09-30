import { createContext } from 'react';

// A renderer capability, not authorization. The Edge validates every read.
export const FileLinkContext = createContext<
  ((href: string) => void) | undefined
>(undefined);
export function isFileReference(href: string) {
  if (!href || href.startsWith('#') || href.startsWith('//')) return false;
  return (
    !/^[a-z][a-z\d+.-]*:/i.test(href) ||
    /^file:/i.test(href) ||
    /^[a-z]:[\\/]/i.test(href) ||
    /^[^:]+\.[a-z\d]+:\d+(?::\d+)?$/i.test(href)
  );
}
