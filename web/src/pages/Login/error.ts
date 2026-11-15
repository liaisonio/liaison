type Translate = (zh: string, en: string) => string;

// Use stable error codes, never server-provided messages on the public login page.
export function loginError(error: unknown, tr: Translate): string {
  const value = error as { code?: number; reason?: string; response?: { status?: number; data?: { code?: number; reason?: string } } } | null;
  const payload = value?.response?.data || value;
  const status = value?.response?.status ?? payload?.code;
  if (status === 401 || payload?.reason === 'INVALID_PASSWORD' || payload?.reason === 'USER_NOT_FOUND') {
    return tr('邮箱或密码错误', 'Incorrect email or password');
  }
  if (payload?.reason === 'ACCOUNT_DISABLED') {
    return tr('账号已禁用，请联系管理员', 'Your account is disabled. Contact your administrator');
  }
  if (status === 429) {
    return tr('登录尝试过于频繁，请稍后重试', 'Too many sign-in attempts. Please try again later');
  }
  return tr('登录失败，请稍后重试', 'Unable to sign in. Please try again later');
}
