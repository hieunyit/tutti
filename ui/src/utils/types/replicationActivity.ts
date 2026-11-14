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

import type { TuttiReplicationId } from '@/utils/types/tuttiCore';

export type ActivityLevel = 'info' | 'warn' | 'error';
export type ActivityType = 'started' | 'done' | 'failed' | 'retry' | 'skipped';

/** int64 fields arrive as strings (grpc-gateway JSON) */
type Int64 = string | number;

export interface ReplicationActivityRequest {
  id: TuttiReplicationId;
  /** only events newer than this cursor; empty = the most recent ones */
  afterCursor?: string;
  /** 1..500 */
  limit?: number;
}

export interface ReplicationInFlight {
  key: string;
  versionId?: string;
  size?: Int64;
  startedAt: string;
  flow: string;
  attempt?: number;
  /** bytes of this object transferred so far (initial replication), <= size */
  bytesDone?: Int64;
}

export interface ReplicationActivityEvent {
  /** opaque, monotonically increasing */
  cursor: string;
  time: string;
  level: ActivityLevel;
  type: ActivityType;
  key: string;
  versionId?: string;
  size?: Int64;
  durationMs?: Int64;
  bytesPerSecond?: Int64;
  attempt?: number;
  flow?: string;
  error?: string;
}

export interface ReplicationActivityResponse {
  inFlight?: ReplicationInFlight[];
  events?: ReplicationActivityEvent[];
  nextCursor?: string;
  serverTime?: string;
}
