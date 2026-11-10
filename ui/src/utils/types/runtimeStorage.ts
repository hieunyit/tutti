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
  /** per-user overrides only */
  users?: { user: string; accessKeyId: string; source: string }[];
  references?: string[];
  createdAt?: string;
  updatedAt?: string;
  connectionAccount?: StorageConnectionAccountInfo;
  lastTest?: StorageTestResult;
  lastForcedSaveBy?: string;
  lastForcedSaveAt?: string;
  /** no connection account: only per-user credentials work */
  legacyCredentials?: boolean;
}

export interface StorageConnectionAccountInfo {
  configured?: boolean;
  accessKeyId?: string;
  source?: 'config' | 'managed' | string;
  previousAccessKeyId?: string;
  rotatedAt?: string;
  safeToRevokePreviousAfter?: string;
}

/** The secret is only ever sent, never returned. */
export interface StorageConnectionAccount {
  accessKeyId: string;
  secretAccessKey: string;
}

export interface StorageTestFeatures {
  versioning: boolean;
  tagging: boolean;
  acl: boolean;
  lifecycle: boolean;
  policy: boolean;
}

export interface StorageTestOptions {
  probeBucketPrefix?: string;
  probeBucket?: string;
  features?: StorageTestFeatures;
  throughput?: boolean;
}

export type StorageTestStatus = 'pass' | 'fail' | 'warn' | 'skip';

export interface StorageTestStep {
  id: string;
  number: number;
  name: string;
  status: StorageTestStatus | string;
  call?: string;
  /** int64 arrives as a string */
  latencyMs?: number | string;
  message?: string;
  missingPermission?: string;
  hint?: string;
}

export interface StorageTestResult {
  testedAt?: string;
  ok: boolean;
  hasWarnings?: boolean;
  steps?: StorageTestStep[];
  probeBucket?: string;
  leftovers?: string[];
  durationMs?: number | string;
  accessKeyId?: string;
  /** "full" or "periodic" (steps 1-5 by the health checker) */
  kind?: string;
}

export interface StorageTestRequest {
  name?: string;
  storage?: RuntimeStorage;
  connectionAccount?: StorageConnectionAccount;
  options?: StorageTestOptions;
}

export interface StorageRotateRequest {
  name: string;
  newAccount: StorageConnectionAccount;
  options?: StorageTestOptions;
  force?: boolean;
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
  connectionAccount?: StorageConnectionAccount;
  clearConnectionAccount?: boolean;
  testOptions?: StorageTestOptions;
}

/** Old per-user credential test (POST /tutti/storages/test-credential). */

export interface RuntimeStorageTestResult {
  ok: boolean;
  error?: string;
  latencyMs?: number | string;
}
