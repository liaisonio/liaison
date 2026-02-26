// 应用名称 - 统一在此处修改
export const APP_NAME = 'Liaison';

export const DEFAULT_NAME = 'Umi Max';

// 默认头像，?v=2 用于缓存失效（更换 avatar.svg 后 bump 版本）
export const DEFAULT_AVATAR = '/avatar.svg?v=2';

/** 头像 URL：优先 DB，否则用默认头像 */
export function getAvatarUrl(avatarFromDb?: string | null): string {
  if (avatarFromDb && avatarFromDb.trim()) return avatarFromDb.trim();
  return DEFAULT_AVATAR;
}

// GitHub 地址
export const GITHUB_URL = 'https://github.com/singchia/liaison';