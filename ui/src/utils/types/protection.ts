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

/** Protection group API (proto/tutti/protection_groups.proto, grpc-gateway JSON). */

export type ProtectionMode = 'manual' | 'auto';

export interface ProtectionGroupEvent {
  from?: string;
  to?: string;
  /** int64 arrives as a string */
  generation?: string | number;
  /** RFC 3339 */
  at?: string;
  automatic?: boolean;
  eventLag?: number;
}

export interface ProtectionGroup {
  id: string;
  user?: string;
  buckets?: string[];
  /** ordered; scope and members are immutable after creation */
  members?: string[];
  copies?: number;
  maxEventLag?: number;
  mode?: ProtectionMode | string;
  /** protobuf duration, "30s" */
  after?: string;
  electorate?: string[];
  maxSwitchesPerHour?: number;
  primary?: string;
  generation?: string | number;
  /** optimistic concurrency token, must be echoed on update */
  revision?: string | number;
  state?: string;
  target?: string;
  frozen?: boolean;
  history?: ProtectionGroupEvent[];
  lastError?: string;
  quarantined?: string[];
}

export interface ProtectionMemberStatus {
  storage: string;
  available?: boolean;
  initialized?: boolean;
  eventLag?: number;
  /** available, initialized, lag 0, verified and not quarantined */
  current?: boolean;
}

export interface ProtectionGroupStatus {
  group?: ProtectionGroup;
  members?: ProtectionMemberStatus[];
  healthyCopies?: number;
  /** int64 as string */
  activeWriters?: string | number;
}

export interface ProtectionGroupListResponse {
  groups?: ProtectionGroup[];
  /** same objects as the per-id GET, same order as `groups` (new backend) */
  statuses?: ProtectionGroupStatus[];
}

export interface ProtectionGroupCreateRequest {
  group: ProtectionGroup;
  adoptExisting?: boolean;
}

export interface ProtectionWriters {
  /** token -> "<owner>:<generation>:<instance>:<admittedUnixMs>" */
  writers?: Record<string, string>;
  workers?: Record<string, string>;
}

export interface ProtectionWriterClearRequest {
  id: string;
  token: string;
  externallyFenced: boolean;
  worker: boolean;
}

/** Editable form (create: all fields, edit: only the mutable ones). */
export interface ProtectionGroupForm {
  id: string;
  user: string | null;
  /** empty = all buckets of the user */
  buckets: string[];
  /** order matters: the first member becomes the primary */
  members: string[];
  copies: number;
  maxEventLag: number | null;
  mode: ProtectionMode;
  /** seconds, null = backend default (30) */
  afterSeconds: number | null;
  electorate: string[];
  maxSwitchesPerHour: number | null;
  adoptExisting: boolean;
}

export type ProtectionFormError =
  | 'ID_REQUIRED'
  | 'ID_INVALID'
  | 'USER_REQUIRED'
  | 'MEMBERS_MIN'
  | 'COPIES_RANGE'
  | 'BUCKET_INVALID'
  | 'LAG_NEGATIVE'
  | 'ELECTORATE_MIN'
  | 'SWITCHES_POSITIVE'
  | 'AFTER_NEGATIVE';

export type ProtectionDisplayState =
  | 'ready'
  | 'initializing'
  | 'waiting'
  | 'draining'
  | 'promoting'
  | 'frozen'
  | 'error'
  | 'unknown';
