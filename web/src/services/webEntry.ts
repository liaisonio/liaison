import {request} from '@/api/client';

export type WebEntryMode = 'path' | 'port' | 'domain';
export const webEntryCapabilities = () => request<API.Response<{domain:boolean}>>('/api/v1/web-entries/capabilities');
export const launchWebEntry = (id:number) => request<API.Response<{url:string}>>(`/api/v1/web-entries/${id}/launch`,{method:'POST',data:{}});
