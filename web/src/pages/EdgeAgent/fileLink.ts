export type FileTarget = { path: string; line?: number; endLine?: number };

export function resolveFileLink(
  href: string,
  project: string,
): FileTarget | undefined {
  try {
    let path = decodeURIComponent(href.trim());
    if (/[\x00-\x1f\x7f]/.test(path) || path.startsWith('//')) return;
    if (/^file:/i.test(path)) {
      const url = new URL(path);
      if (url.hostname && url.hostname !== 'localhost') return;
      path = decodeURIComponent(url.pathname) + url.hash;
      if (/^\/[A-Za-z]:\//.test(path)) path = path.slice(1);
    }
    const location = path.match(
      /(?:#L(\d+)(?:C\d+)?(?:-L?(\d+)(?:C\d+)?)?|:(\d+)(?::\d+)?)$/,
    );
    let line: number | undefined, endLine: number | undefined;
    if (location) {
      line = Number(location[1] || location[3]);
      endLine = Number(location[2] || line);
      path = path.slice(0, location.index);
      if (
        !Number.isSafeInteger(line) ||
        line < 1 ||
        !Number.isSafeInteger(endLine) ||
        endLine < line
      )
        return;
    }
    path = path.replace(/\\/g, '/');
    const root = project.replace(/\\/g, '/').replace(/\/+$/, '');
    if (path.startsWith('/') || /^[A-Za-z]:\//.test(path)) {
      if (!root) return;
      const windows = /^[A-Za-z]:\//.test(root);
      const candidate = windows ? path.toLowerCase() : path,
        base = windows ? root.toLowerCase() : root;
      if (candidate !== base && !candidate.startsWith(base + '/')) return;
      path = path.slice(root.length).replace(/^\//, '') || '.';
    }
    if (
      /[?#:]/.test(path) ||
      path.split('/').some((p) => p === '..' || p.startsWith('.upload-'))
    )
      return;
    path =
      path
        .split('/')
        .filter((p) => p && p !== '.')
        .join('/') || '.';
    return { path, line, endLine };
  } catch {
    return;
  }
}
