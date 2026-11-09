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
