/**
 * 通用表格配置
 * 统一的表格默认配置和常用列定义
 */

import { ProColumns } from '@ant-design/pro-components';
import { tr } from '@/i18n';

/**
 * 默认分页配置
 */
export const defaultPagination = {
  defaultPageSize: 10,
  showSizeChanger: true,
  showQuickJumper: true,
  showTotal: (total: number, range: [number, number]) =>
    tr(`第 ${range[0]}-${range[1]} 条/总共 ${total} 条`, `${range[0]}-${range[1]} of ${total} items`),
};

/**
 * 默认搜索配置
 */
export const defaultSearch = {
  labelWidth: 'auto' as const,
  span: 6, // 每行显示4个字段（24/6=4）
  defaultCollapsed: true, // 默认折叠搜索表单
  searchText: '搜索',
  resetText: '重置',
};

/**
 * 创建时间列配置
 */
export const createdAtColumn: ProColumns<any> = {
  title: '创建时间',
  dataIndex: 'created_at',
  valueType: 'dateTime',
  width: 180,
  search: false,
};

/**
 * 更新时间列配置
 */
export const updatedAtColumn: ProColumns<any> = {
  title: '更新时间',
  dataIndex: 'updated_at',
  valueType: 'dateTime',
  width: 180,
  search: false,
};

/**
 * 描述列配置
 */
export const descriptionColumn: ProColumns<any> = {
  title: '描述',
  dataIndex: 'description',
  ellipsis: true,
  search: false,
};

/**
 * 构建搜索参数
 * 过滤空值，只保留有效的搜索参数
 */
export function buildSearchParams<T extends Record<string, any>>(
  params: Record<string, any>,
  fields: string[],
): T {
  const result: Record<string, any> = {
    page: params.current,
    page_size: params.pageSize,
  };

  fields.forEach((field) => {
    if (params[field] !== undefined && params[field] !== null && params[field] !== '') {
      result[field] = params[field];
    }
  });

  return result as T;
}
