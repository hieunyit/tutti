/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

export interface TuttiStorageHealth {
  name: string;
  available?: boolean;
  consecutiveFailures?: number;
  lastError?: string;
  lastChange?: string;
}

export interface TuttiSyncSettings {
  enabled?: boolean;
  writeQuorum?: number;
  timeout?: string;
  onQuorumFailure?: string;
}

export interface TuttiProxySettings {
  replication?: {
    onEventError?: string;
    eventRetryBudget?: string;
    sync?: TuttiSyncSettings;
  };
  health?: {
    failureThreshold?: number;
    openDuration?: string;
  };
  failover?: {
    enabled?: boolean;
    checkInterval?: string;
    after?: string;
    maxEventLag?: number;
  };
  requireSignedContentType?: boolean;
}

export interface TuttiProxyStatus {
  appId: string;
  address?: string;
  startedAt?: string;
  updatedAt?: string;
  storages?: TuttiStorageHealth[];
  settings?: TuttiProxySettings;
  /** absent for proxies that do not publish traffic stats */
  stats?: TuttiTrafficStats;
}

export interface TuttiProxyStatusListResponse {
  proxies?: TuttiProxyStatus[];
}

export interface TuttiFailoverPolicy {
  source: string;
  enabled?: boolean;
  targets?: string[];
  /** protobuf duration string, e.g. "60s" */
  after?: string;
  /** undefined means "use the proxy default" */
  maxEventLag?: number;
}

export interface TuttiFailoverPolicyListResponse {
  policies?: TuttiFailoverPolicy[];
}

export interface TuttiFailoverEvent {
  user: string;
  /** empty means all buckets of the user */
  bucket?: string;
  from: string;
  to: string;
  time?: string;
  eventLag?: number;
}

export interface TuttiFailoverEventListResponse {
  events?: TuttiFailoverEvent[];
}

export interface TuttiFailoverEventId {
  user: string;
  bucket: string;
}

export interface TuttiFailoverPolicyForm {
  source: string | null;
  enabled: boolean;
  targets: string[];
  /** raw user input: seconds or "60s"/"5m"/"1h" */
  after: string;
  /** null means "use proxy default" */
  maxEventLag: number | null;
}

export enum TuttiPolicyValidationError {
  SOURCE_REQUIRED = 'SOURCE_REQUIRED',
  TARGETS_REQUIRED = 'TARGETS_REQUIRED',
  TARGET_IS_SOURCE = 'TARGET_IS_SOURCE',
  TARGET_DUPLICATE = 'TARGET_DUPLICATE',
  AFTER_INVALID = 'AFTER_INVALID',
  MAX_EVENT_LAG_INVALID = 'MAX_EVENT_LAG_INVALID',
}

export enum TuttiAccessPermission {
  READ_WRITE = 'read-write',
  READ_ONLY = 'read-only',
  WRITE_ONLY = 'write-only',
}

export interface TuttiAccessPolicy {
  user: string;
  permission?: string;
  /** glob patterns; empty or ["*"] means all buckets */
  buckets?: string[];
  description?: string;
}

export interface TuttiAccessPolicyListResponse {
  policies?: TuttiAccessPolicy[];
}

export interface TuttiProxyAccount {
  user: string;
  accessKeyIds?: string[];
  /** parallel to accessKeyIds: "custom" or "storage:<name>" */
  sources?: string[];
  /** effective policy (read-write on "*" when none is stored) */
  policy?: TuttiAccessPolicy;
  hasPolicy?: boolean;
}

export interface TuttiProxyAccountListResponse {
  accounts?: TuttiProxyAccount[];
}

export interface TuttiAccessKey {
  accessKeyId: string;
  source: string;
}

export interface TuttiAccessPolicyForm {
  user: string;
  permission: TuttiAccessPermission;
  buckets: string[];
  description: string;
}

export enum TuttiAccessPolicyValidationError {
  USER_REQUIRED = 'USER_REQUIRED',
  PERMISSION_INVALID = 'PERMISSION_INVALID',
  PATTERN_EMPTY = 'PATTERN_EMPTY',
  PATTERN_WHITESPACE = 'PATTERN_WHITESPACE',
  PATTERN_DUPLICATE = 'PATTERN_DUPLICATE',
}

/** int64 values arrive as strings in protobuf JSON */
export type TuttiInt64 = string | number;

export interface TuttiLatencyHistogram {
  count?: TuttiInt64;
  sumSeconds?: number;
  boundsSeconds?: number[];
  /** non-cumulative; the last bucket is +Inf */
  buckets?: TuttiInt64[];
}

export interface TuttiTrafficStats {
  collectedAt?: string;
  startedAt?: string;
  requestsTotal?: TuttiInt64;
  requestsByClass?: Record<string, TuttiInt64>;
  requestsByStorage?: Record<string, TuttiInt64>;
  responses2xx?: TuttiInt64;
  responses3xx?: TuttiInt64;
  responses4xx?: TuttiInt64;
  responses5xx?: TuttiInt64;
  responsesOther?: TuttiInt64;
  failFast?: TuttiInt64;
  accessDenied?: TuttiInt64;
  eventRecordFailed?: TuttiInt64;
  syncQuorumReached?: TuttiInt64;
  syncBelowQuorum?: TuttiInt64;
  syncSkipped?: TuttiInt64;
  chunkedVerifyFailed?: TuttiInt64;
  bytesIn?: TuttiInt64;
  bytesOut?: TuttiInt64;
  latency?: TuttiLatencyHistogram;
}
