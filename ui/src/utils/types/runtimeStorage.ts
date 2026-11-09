/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

export interface RuntimeStorageCredential {
  user: string;
  accessKeyId: string;
  secretAccessKey: string;
}

export interface RuntimeStorage {
  name: string;
  address: string;
  provider: string;
  defaultRegion: string;
  isSecure: boolean;
  httpTimeout: string;
  healthCheckInterval: string;
  rateLimit?: { enabled: boolean; rpm: number; includeMetadataApi: boolean };
  source?: 'config' | 'managed';
  isMain?: boolean;
  users?: { user: string; accessKeyId: string; source: string }[];
  references?: string[];
  createdAt?: string;
  updatedAt?: string;
}

export interface RuntimeStorageList {
  storages?: RuntimeStorage[];
  mainStorage?: string;
  managedStoragesEnabled?: boolean;
}

export interface RuntimeStorageMutation {
  storage: RuntimeStorage;
  credentials?: RuntimeStorageCredential[];
  force?: boolean;
  confirm?: boolean;
}

export interface RuntimeStorageTestResult {
  ok: boolean;
  error?: string;
  latencyMs?: number | string;
}
