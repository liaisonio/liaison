import { isSQLProtocol } from './protocol';

export const statementDraftPrefix = 'liaison:webdata:statement:v1';
export const anonymousStatementUser = 'anonymous';

export const currentUserStorageKey = (user?: API.CurrentUser) => {
  if (user?.id) return `user-${user.id}`;
  return user?.email || user?.name || anonymousStatementUser;
};

export const objectStatementDraftKey = (
  userKey: string,
  proxyId: number,
  protocol?: string,
  detail?: API.WebDataObjectResult,
) => {
  if (!isSQLProtocol(protocol) || !detail) return '';
  const objectName = detail.key || detail.name || '';
  const parts = [
    statementDraftPrefix,
    userKey || anonymousStatementUser,
    String(proxyId || 0),
    protocol || '',
    detail.object_type || '',
    detail.database || '',
    detail.schema || '',
    objectName,
  ];
  if (!objectName && !detail.database && !detail.schema) return '';
  return parts.map((part) => encodeURIComponent(part)).join(':');
};

export const readStatementDraft = (key: string) => {
  if (!key || typeof window === 'undefined') return '';
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return '';
    try {
      const parsed = JSON.parse(raw) as { statement?: unknown };
      const statement =
        typeof parsed.statement === 'string' ? parsed.statement : '';
      if (isDisposableStatementDraft(statement)) {
        window.localStorage.removeItem(key);
        return '';
      }
      return statement;
    } catch {
      if (isDisposableStatementDraft(raw)) {
        window.localStorage.removeItem(key);
        return '';
      }
      return raw;
    }
  } catch {
    return '';
  }
};

export const writeStatementDraft = (key: string, statement: string) => {
  if (!key || typeof window === 'undefined') return;
  try {
    if (!statement.trim() || isDisposableStatementDraft(statement)) {
      window.localStorage.removeItem(key);
      return;
    }
    window.localStorage.setItem(
      key,
      JSON.stringify({
        statement,
        updatedAt: new Date().toISOString(),
      }),
    );
  } catch {}
};

export const isDisposableStatementDraft = (statement: string) =>
  /^select\s+1\s*;?$/i.test(statement.trim());
