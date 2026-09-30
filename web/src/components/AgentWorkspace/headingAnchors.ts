type Node = {
  type: string;
  tagName?: string;
  value?: string;
  properties?: Record<string, unknown>;
  children?: Node[];
};
// IDs are scoped to a document preview, not global page navigation.
export function headingAnchors() {
  return (tree: Node) => {
    const used = new Set<string>();
    const text = (node: Node): string =>
      node.type === 'text'
        ? node.value || ''
        : (node.children || []).map(text).join('');
    const visit = (node: Node) => {
      if (node.tagName && /^h[1-6]$/.test(node.tagName)) {
        const base = text(node)
          .toLowerCase()
          .trim()
          .replace(/[^\p{L}\p{N}\p{M}_\s-]/gu, '')
          .replace(/\s/g, '-');
        let slug = base,
          index = 0;
        while (used.has(slug)) slug = `${base}-${++index}`;
        used.add(slug);
        node.properties = { ...node.properties, id: slug };
      }
      node.children?.forEach(visit);
    };
    visit(tree);
  };
}
