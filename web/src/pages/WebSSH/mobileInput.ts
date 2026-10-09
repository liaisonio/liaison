// One-shot Ctrl modifier for the next terminal character, including Ctrl+Space.
export function controlInput(data: string): string {
  if (data.length !== 1) return data;
  const code = data.charCodeAt(0);
  if (code >= 97 && code <= 122) return String.fromCharCode(code - 96);
  if (code >= 64 && code <= 95) return String.fromCharCode(code - 64);
  if (data === ' ') return '\x00';
  if (data === '?') return '\x7f';
  return data;
}
