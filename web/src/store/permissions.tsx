import { create } from 'zustand';
import { useEffect, type ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { request } from '@/api/client';
import { useSession } from './session';
import { useI18n } from '@/i18n';

type PermissionState = { owner: string | null; loaded: boolean; grants: Record<string, boolean> };
export const usePermissions = create<PermissionState>(() => ({ owner:null, loaded:false, grants:{} }));
let generation = 0;
export async function refreshPermissions() {
  const token = useSession.getState().token;
  const version = ++generation;
  if (!token) { usePermissions.setState({owner:null,loaded:false,grants:{}}); return; }
  try {
    const response = await request<API.Response<Record<string, boolean>>>('/api/v1/iam/permissions');
    if (version === generation && token === useSession.getState().token) usePermissions.setState({owner:token, loaded:true, grants:response.data || {}});
  } catch {
    if (version === generation && token === useSession.getState().token) usePermissions.setState({owner:token,loaded:true,grants:{}});
  }
}
export function PermissionRefresh() {
  const token = useSession(s => s.token);
  useEffect(() => {
    void refreshPermissions();
    const refresh = () => { void refreshPermissions(); };
    window.addEventListener('focus', refresh);
    const timer = window.setInterval(refresh, 20000);
    return () => { window.removeEventListener('focus',refresh); window.clearInterval(timer); };
  }, [token]);
  return null;
}
export function useFeature(code:string) {
  const token = useSession(s => s.token);
  return usePermissions(s => s.owner === token && s.loaded && s.grants[code] === true);
}
export function FeatureGate({code, children, home=false}:{code:string;children:ReactNode;home?:boolean}) {
  const allowed = useFeature(code);
  const loaded = usePermissions(s => s.loaded);
  const {tr} = useI18n();
  if (!loaded) return <div role="status">{tr('加载权限…','Loading permissions…')}</div>;
  if (!allowed) return home ? <Navigate to="/dashboard" replace /> : <div role="alert">{tr('管理员尚未向你开放此功能。','Your administrator has not enabled this feature for you.')}</div>;
  return <>{children}</>;
}
