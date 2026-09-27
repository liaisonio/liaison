import { resolveFileLink, type FileTarget } from './fileLink';

// Keep fragments separate from the filesystem path. File authorization stays on Edge.
export function previewTarget(
  href: string,
  project: string,
): (FileTarget & { anchor?: string }) | undefined {
  const hash = href.indexOf('#'),
    fragment = hash < 0 ? '' : href.slice(hash + 1);
  if (!fragment || /^L\d+(?:C\d+)?(?:-L?\d+(?:C\d+)?)?$/.test(fragment))
    return resolveFileLink(href, project);
  try {
    const target = resolveFileLink(href.slice(0, hash), project),
      anchor = decodeURIComponent(fragment);
    if (!target || anchor.length > 256 || /[\x00-\x1f\x7f]/.test(anchor))
      return;
    return { ...target, anchor };
  } catch {
    return;
  }
}

export function documentLink(
  href: string,
  current: string,
  project: string,
): string | undefined {
  try {
    const hash = href.indexOf('#');
    let suffix = hash < 0 ? '' : href.slice(hash);
    const raw = hash < 0 ? href : href.slice(0, hash);
    let decoded = decodeURIComponent(raw).replace(/\\/g, '/');
    let path: string;
    if (!raw) path = current;
    else if (
      decoded.startsWith('/') ||
      /^file:/i.test(decoded) ||
      /^[a-z]:\//i.test(decoded)
    ) {
      const absolute = resolveFileLink(raw, project);
      if (!absolute) return;
      path = absolute.path;
      // Preserve explicit source line targets on absolute links.
      if (absolute.line)
        return (
          path.split('/').map(encodeURIComponent).join('/') +
          `#L${absolute.line}-L${absolute.endLine || absolute.line}`
        );
    } else {
      const line = decoded.match(/:(\d+)(?::\d+)?$/);
      if (line && !suffix) {
        suffix = `#L${line[1]}`;
        decoded = decoded.slice(0, line.index);
      }
      if (/^[a-z][a-z\d+.-]*:/i.test(decoded)) return;
      const parts = current.split('/').slice(0, -1);
      for (const part of decoded.split('/')) {
        if (!part || part === '.') continue;
        if (part === '..') {
          if (!parts.length) return;
          parts.pop();
        } else parts.push(part);
      }
      path = parts.join('/') || '.';
    }
    const candidate =
      path.split('/').map(encodeURIComponent).join('/') + suffix;
    return previewTarget(candidate, project) ? candidate : undefined;
  } catch {
    return;
  }
}
