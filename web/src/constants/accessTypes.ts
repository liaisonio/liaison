import type { ApplicationType } from './applicationTypes';

export type WebAccessType =
  | 'webssh'
  | 'webrdp'
  | 'webvnc'
  | 'webmysql'
  | 'webpostgresql'
  | 'webredis'
  | 'webmongodb';

export type AccessType = ApplicationType | WebAccessType;

type AccessTypeOption = { value: AccessType; label: string };

const NATIVE_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'http', label: 'HTTP' },
  { value: 'ssh', label: 'SSH' },
  { value: 'rdp', label: 'RDP' },
  { value: 'vnc', label: 'VNC' },
];

// Kept as known values so legacy records can still be identified and hidden
// without being misclassified as TCP. Liaison does not currently terminate
// these native database protocols, so they are not exposed as product access
// types until a protocol-aware server implementation exists.
const UNSUPPORTED_NATIVE_DATA_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'mysql', label: 'MySQL' },
  { value: 'postgresql', label: 'PostgreSQL' },
  { value: 'redis', label: 'Redis' },
  { value: 'mongodb', label: 'MongoDB' },
];

const WEB_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'webssh', label: 'Web SSH' },
  { value: 'webrdp', label: 'Web RDP' },
  { value: 'webvnc', label: 'Web VNC' },
  { value: 'webmysql', label: 'Web MySQL' },
  { value: 'webpostgresql', label: 'Web PostgreSQL' },
  { value: 'webredis', label: 'Web Redis' },
  { value: 'webmongodb', label: 'Web MongoDB' },
];

// Keep one product-wide order: L4 passthrough, native protocol, browser access.
export const ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'tcp', label: 'TCP' },
  ...NATIVE_ACCESS_TYPES,
  ...WEB_ACCESS_TYPES,
];

const KNOWN_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  ...ACCESS_TYPES,
  ...UNSUPPORTED_NATIVE_DATA_ACCESS_TYPES,
];

const WEB_TYPE_BY_APPLICATION: Partial<Record<ApplicationType, WebAccessType>> = {
  ssh: 'webssh',
  rdp: 'webrdp',
  vnc: 'webvnc',
  mysql: 'webmysql',
  postgresql: 'webpostgresql',
  redis: 'webredis',
  mongodb: 'webmongodb',
};

export const ACCESS_TYPES_CHANGED_EVENT = 'liaison:access-types-changed';

export const isAccessType = (
  value: string | null | undefined,
): value is AccessType =>
  KNOWN_ACCESS_TYPES.some((accessType) => accessType.value === value);

export const isSupportedAccessType = (
  value: string | null | undefined,
) => ACCESS_TYPES.some((accessType) => accessType.value === value);

export const accessTypeLabel = (value: string | null | undefined) =>
  KNOWN_ACCESS_TYPES.find((item) => item.value === value)?.label || value || '-';

export const isWebAccessType = (
  value: string | null | undefined,
): value is WebAccessType => WEB_ACCESS_TYPES.some((item) => item.value === value);

export const applicationTypeForAccess = (accessType: AccessType): ApplicationType => {
  const webEntry = Object.entries(WEB_TYPE_BY_APPLICATION).find(([, value]) => value === accessType);
  return (webEntry?.[0] || accessType) as ApplicationType;
};

export const accessProtocolForType = (accessType: AccessType) => {
  if (accessType === 'webssh') return 'webssh';
  return isWebAccessType(accessType) ? 'web' : accessType;
};

export const accessTypesForApplication = (applicationType: string): AccessTypeOption[] => {
  const native = NATIVE_ACCESS_TYPES.find((item) => item.value === applicationType);
  const webType = WEB_TYPE_BY_APPLICATION[applicationType as ApplicationType];
  return [
    { value: 'tcp', label: 'TCP' },
    ...(applicationType !== 'tcp' && native ? [native] : []),
    ...(webType ? WEB_ACCESS_TYPES.filter((item) => item.value === webType) : []),
  ];
};

export const isProxyPublicPortExposed = (proxy?: API.Proxy) =>
  Boolean(proxy?.expose_public_port ?? (proxy?.port || 0) > 0);

export const getProxyAccessType = (
  proxy?: API.Proxy,
): AccessType | undefined => {
  const accessProtocol = String(proxy?.access_protocol || '').toLowerCase();
  const applicationType = String(
    proxy?.application?.application_type || '',
  ).toLowerCase();
  if (!applicationType) return undefined;
  if (accessProtocol === 'web') {
    return WEB_TYPE_BY_APPLICATION[applicationType as ApplicationType];
  }
  if (accessProtocol === 'webssh') return 'webssh';
  if (isAccessType(accessProtocol)) return accessProtocol;
  if (applicationType === 'ssh' && !isProxyPublicPortExposed(proxy)) {
    return 'webssh';
  }
  // Legacy records do not have access_protocol. Keep their historical routing.
  if (applicationType !== 'http' && isProxyPublicPortExposed(proxy)) {
    return 'tcp';
  }
  return isAccessType(applicationType) ? applicationType : undefined;
};
