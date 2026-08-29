/**
 * 通用格式化工具函数
 */

/**
 * 格式化字节大小
 * @param bytes 字节数
 * @returns 格式化后的字符串，如 "1.5 GB"
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

/**
 * 格式化 MB 为人类可读的大小
 * @param mb MB 数值
 * @returns 格式化后的字符串，如 "960 MB" 或 "39.26 GB"
 */
export function formatMBSize(mb: number): string {
  if (mb === 0) return '0 MB';
  if (mb < 1024) {
    return `${mb.toFixed(0)} MB`;
  }
  // 转换为 GB
  const gb = mb / 1024;
  if (gb < 1024) {
    return `${gb.toFixed(2)} GB`;
  }
  // 转换为 TB
  const tb = gb / 1024;
  return `${tb.toFixed(2)} TB`;
}


/**
 * 去除字符串首尾空格
 */
export function trim(str: string) {
  return str.trim();
}
