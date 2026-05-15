import { isSQLProtocol, protocolLabels } from './protocol';

export const defaultConnectionName = (protocol?: string) =>
  `${protocolLabels[String(protocol || '')] || protocol || 'Database'} 连接`;

export const connectionTitle = (credential: API.WebDataCredential) =>
  credential.name ||
  [
    credential.username || 'anonymous',
    credential.protocol === 'redis'
      ? `db${credential.redis_db ?? 0}`
      : credential.database,
  ]
    .filter(Boolean)
    .join(' / ') ||
  defaultConnectionName(credential.protocol);

export const connectionSubtitle = (credential: API.WebDataCredential) => {
  if (credential.protocol === 'redis') {
    return `Redis DB ${credential.redis_db ?? 0}`;
  }
  if (credential.protocol === 'mongodb') {
    return credential.auth_database
      ? `${credential.database || 'default'} · auth ${credential.auth_database}`
      : credential.database || 'default';
  }
  return credential.database || 'default';
};

export const connectionMeta = (credential: API.WebDataCredential) => {
  const items = [
    credential.username ? `user: ${credential.username}` : 'no user',
  ];
  if (credential.protocol === 'redis') {
    items.splice(1, 0, `db: ${credential.redis_db ?? 0}`);
  } else if (credential.database) {
    items.splice(1, 0, `db: ${credential.database}`);
  }
  if (isSQLProtocol(credential.protocol)) {
    items.push(
      credential.tls_mode && credential.tls_mode !== 'disable'
        ? `TLS: ${credential.tls_mode}`
        : 'TLS: off',
    );
    if (credential.schema) {
      items.push(`schema: ${credential.schema}`);
    }
  }
  if (credential.protocol === 'redis') {
    items.push(
      credential.tls_mode && credential.tls_mode !== 'disable'
        ? `TLS: ${credential.tls_mode}`
        : 'TLS: off',
    );
  }
  if (credential.protocol === 'mongodb') {
    if (credential.auth_mechanism) {
      items.push(`auth: ${credential.auth_mechanism}`);
    }
    items.push(
      credential.direct_connection === false ? 'direct: off' : 'direct: on',
    );
    items.push(
      credential.tls_mode && credential.tls_mode !== 'disable'
        ? `TLS: ${credential.tls_mode}`
        : 'TLS: off',
    );
  }
  if (credential.connection_params) {
    items.push('params');
  }
  return items;
};

export const normalizeConnectionTLSMode = (tlsMode?: string) => {
  const value = String(tlsMode || '')
    .trim()
    .toLowerCase();
  if (value === 'true') return 'require';
  if (['require', 'skip-verify', 'preferred'].includes(value)) return value;
  return 'disable';
};

export const hasFormField = (values: any, key: string) =>
  Object.prototype.hasOwnProperty.call(values || {}, key);

export const formStringOrCredential = (
  values: any,
  key: string,
  credentialValue?: string,
) => {
  if (!hasFormField(values, key)) return credentialValue;
  return values[key]?.trim();
};

export const formValueOrCredential = <T>(
  values: any,
  key: string,
  credentialValue: T | undefined,
  fallback: T,
) => {
  if (!hasFormField(values, key)) return credentialValue ?? fallback;
  return values[key] ?? fallback;
};

export const buildConnectionProfileFromValues = (
  protocol: string,
  values: any,
  options?: { credential?: API.WebDataCredential },
): API.SaveWebDataCredentialRequest => {
  const credential = options?.credential;
  const tlsMode = hasFormField(values, 'tls_mode')
    ? normalizeConnectionTLSMode(values.tls_mode)
    : normalizeConnectionTLSMode(credential?.tls_mode);
  return {
    protocol,
    username: formStringOrCredential(values, 'username', credential?.username),
    database: formStringOrCredential(values, 'database', credential?.database),
    auth_database: formStringOrCredential(
      values,
      'auth_database',
      credential?.auth_database,
    ),
    redis_db: formValueOrCredential(
      values,
      'redis_db',
      credential?.redis_db,
      0,
    ),
    tls_mode: tlsMode,
    schema: formStringOrCredential(values, 'schema', credential?.schema),
    auth_mechanism: formValueOrCredential(
      values,
      'auth_mechanism',
      credential?.auth_mechanism,
      '',
    ),
    direct_connection: formValueOrCredential(
      values,
      'direct_connection',
      credential?.direct_connection,
      true,
    ),
    connection_params: formStringOrCredential(
      values,
      'connection_params',
      credential?.connection_params,
    ),
  };
};

export const buildConnectionRequestFromValues = (
  protocol: string,
  values: any,
  options?: {
    credential?: API.WebDataCredential;
    credentialId?: number;
    saveCredential?: boolean;
  },
): API.CreateWebDataSessionRequest => ({
  credential_id: options?.credentialId,
  protocol,
  password: values.password,
  ...buildConnectionProfileFromValues(protocol, values, {
    credential: options?.credential,
  }),
  save_credential: options?.saveCredential,
});

export const tlsOptionsForProtocol = (
  protocol: string,
  tr: (zh: string, en: string) => string,
) => {
  if (protocol === 'mysql') {
    return [
      { label: tr('关闭', 'Disabled'), value: 'disable' },
      { label: 'MySQL skip-verify', value: 'skip-verify' },
      { label: 'MySQL preferred', value: 'preferred' },
    ];
  }
  return [
    { label: tr('关闭', 'Disabled'), value: 'disable' },
    { label: tr('启用/要求', 'Enabled/Required'), value: 'require' },
    { label: 'skip-verify', value: 'skip-verify' },
  ];
};
