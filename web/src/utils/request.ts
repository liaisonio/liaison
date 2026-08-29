/**
 * 统一请求封装
 * 自动处理 code === 200 的判断逻辑
 */

import { message } from 'antd';

/** 成功响应码 */
const SUCCESS_CODE = 200;

/**
 * 执行带成功/失败提示的操作
 */
export async function executeAction<T = any>(
  action: () => Promise<API.Response<T>>,
  options?: {
    successMessage?: string;
    errorMessage?: string;
    onSuccess?: (data?: T) => void;
  },
): Promise<boolean> {
  const {
    successMessage = '操作成功',
    errorMessage = '操作失败',
    onSuccess,
  } = options || {};

  try {
    const res = await action();
    if (res.code === SUCCESS_CODE) {
      if (successMessage) {
        message.success(successMessage);
      }
      onSuccess?.(res.data);
      return true;
    }
    message.error(res.message || errorMessage);
    return false;
  } catch (error: any) {
    message.error(error?.message || errorMessage);
    return false;
  }
}

/**
 * 用于 ProTable 的 request 封装
 * 自动处理分页和成功判断
 */
export async function tableRequest<T = any>(
  fetcher: () => Promise<API.Response<any>>,
  dataKey: string,
): Promise<{
  data: T[];
  total: number;
  success: boolean;
}> {
  try {
    const res = await fetcher();
    if (res.code !== SUCCESS_CODE) {
      return {
        data: [],
        total: 0,
        success: false,
      };
    }
    const data = res.data?.[dataKey] || [];
    return {
      data,
      total: res.data?.total || 0,
      success: true,
    };
  } catch {
    return {
      data: [],
      total: 0,
      success: false,
    };
  }
}
