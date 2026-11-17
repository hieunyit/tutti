// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

export interface LifecycleRunStats {
  deletedCount: string;
  bytesFreed: string;
  durationMs: string;
}

export interface LifecycleRule {
  id: string;
  name: string;
  storageName: string;
  bucket: string;
  prefix: string;
  expirationDays: number;
  schedule: string;
  enabled: boolean;
  nextRunAt?: string;
  lastRunAt?: string;
  lastRunStats?: LifecycleRunStats;
  createdAt: string;
  updatedAt: string;
}

export interface CreateLifecycleRuleRequest {
  name: string;
  storageName: string;
  bucket: string;
  prefix?: string;
  expirationDays: number;
  schedule?: string;
  enabled?: boolean;
}

export interface ListLifecycleRulesRequest {
  storageName?: string;
  bucket?: string;
}

export interface ListLifecycleRulesResponse {
  rules?: LifecycleRule[];
}

export interface UpdateLifecycleRuleRequest {
  id: string;
  name: string;
  prefix?: string;
  expirationDays: number;
  schedule?: string;
  enabled?: boolean;
}

export interface LifecycleDryRunRequest {
  storageName: string;
  bucket: string;
  prefix?: string;
  expirationDays: number;
  sampleSize?: number;
}

export interface LifecycleDryRunResponse {
  estimatedDeletedCount: string;
  estimatedBytesFreed: string;
  sampleObjects?: string[];
}

export interface LifecycleAuditLogEntry {
  id: string;
  ruleId: string;
  bucket: string;
  objectKey: string;
  sizeBytes: string;
  lastModified: string;
  deletedAt: string;
}

export interface ListLifecycleAuditLogsResponse {
  entries?: LifecycleAuditLogEntry[];
  total: string;
}
