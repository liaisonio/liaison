import { getToken, useSession } from '@/store/session';

export type AccessDraft = { accessId: number; name: string; prompt: string };
// Draft text stays in this tab's memory, never in URLs or persistent storage.
const pending = new Map<string, AccessDraft & { owner: string; expires: number }>();
useSession.subscribe((state, previous) => { if (state.token !== previous.token) pending.clear(); });
export function stageAccessDraft(draft: AccessDraft) {
  const owner = getToken();
  if (!owner || !Number.isSafeInteger(draft.accessId) || draft.accessId <= 0 || !draft.prompt.trim()) return undefined;
  for (const [id, entry] of pending) if (entry.expires < Date.now()) pending.delete(id);
  if (pending.size >= 20) pending.delete(pending.keys().next().value!);
  const id = crypto.randomUUID();
  pending.set(id, {...draft, prompt: draft.prompt.slice(0, 12000), owner, expires: Date.now() + 10 * 60 * 1000});
  return id;
}
export function accessDraft(id: unknown, accessId?: number): AccessDraft | undefined {
  if (typeof id !== 'string') return;
  const entry = pending.get(id);
  if (!entry) return;
  if (entry.owner !== getToken() || entry.expires < Date.now()) { pending.delete(id); return; }
  if (accessId !== undefined && entry.accessId !== accessId) return;
  return {accessId: entry.accessId, name: entry.name, prompt: entry.prompt};
}
export function discardAccessDraft(id: unknown) { if (typeof id === 'string') pending.delete(id); }
