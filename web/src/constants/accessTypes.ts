import { APPLICATION_TYPES, type ApplicationType } from './applicationTypes';

export type AccessType = ApplicationType | 'webssh';

export const ACCESS_TYPES: ReadonlyArray<{
  value: AccessType;
  label: string;
}> = [
  ...APPLICATION_TYPES.slice(0, 3),
  { value: 'webssh', label: 'WebSSH' },
  ...APPLICATION_TYPES.slice(3),
];

export const ACCESS_TYPES_CHANGED_EVENT = 'liaison:access-types-changed';

export const isAccessType = (
  value: string | null | undefined,
): value is AccessType =>
  ACCESS_TYPES.some((accessType) => accessType.value === value);

export const isProxyPublicPortExposed = (proxy?: API.Proxy) =>
  Boolean(proxy?.expose_public_port ?? (proxy?.port || 0) > 0);

export const getProxyAccessType = (
  proxy?: API.Proxy,
): AccessType | undefined => {
  const applicationType = String(
    proxy?.application?.application_type || '',
  ).toLowerCase();
  if (!applicationType) return undefined;
  if (applicationType === 'ssh' && !isProxyPublicPortExposed(proxy)) {
    return 'webssh';
  }
  return isAccessType(applicationType) ? applicationType : undefined;
};
