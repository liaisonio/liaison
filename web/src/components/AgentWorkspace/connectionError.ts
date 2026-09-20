export function connectionError(error: unknown, tr: (zh: string, en: string) => string): string {
  const status = (error as {response?: {status?: number}})?.response?.status;
  if (status === 401) return tr('登录已过期，请重新登录。', 'Your login has expired. Sign in again.');
  if (status === 403) return tr('当前无权查看此会话，请联系管理员。', 'You no longer have access to this session. Contact your administrator.');
  if (status === 404) return tr('会话不存在或已被移除。', 'This session no longer exists.');
  return tr('暂时无法连接服务，请检查网络后重试。已有对话不会被清除。', 'Cannot reach the service. Check your connection and retry. Your conversation is preserved.');
}
