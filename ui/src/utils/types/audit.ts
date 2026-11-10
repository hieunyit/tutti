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

export const AUDIT_LOG_ROUTE_NAME = 'TUTTI_AUDIT_LOG' as const;

export const AUDIT_LOG_FAMILIES = [
  'storage',
  's3_identity',
  'dynamic_credential',
  'proxy_account',
  'access_policy',
  'console_user',
  'route',
  'route_block',
  'replication_policy',
  'replication_status',
  'replication_generation',
  'failover_policy',
  'failover_event',
  'protection_group',
  'storage_test',
  'proxy_fleet_config',
] as const;

export type AuditLogFamily =
  | (typeof AUDIT_LOG_FAMILIES)[number]
  | (string & {});

export const AUDIT_LOG_ACTIONS = [
  'create',
  'update',
  'delete',
  'set',
  'block',
  'unblock',
  'generation',
  'archive',
  'rotate',
] as const;

export type AuditLogAction = (typeof AUDIT_LOG_ACTIONS)[number] | (string & {});

export interface ListAuditLogParams {
  family?: string;
  entityKey?: string;
  actor?: string;
  action?: string;
  beforeId?: string | number;
  limit?: number;
}

export interface AuditLogEntry {
  id: string | number;
  at: string;
  actor: string;
  requestId: string;
  action: string;
  family: string;
  entityKey: string;
  beforeJson?: string;
  afterJson?: string;
}

export interface AuditLogList {
  backend: string; // "redis" | "postgres"
  readOnly: boolean;
  entries: AuditLogEntry[];
}
