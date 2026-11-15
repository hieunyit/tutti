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

import type { StatusTone } from '@/components/kit';
import type { AuditLogEntry } from '@/utils/types/audit';

/** Object types as the control plane stores them (filter values). */
export const AUDIT_FAMILY_FILTERS = [
  'storages',
  's3_identities',
  'proxy_accounts',
  'access_policies',
  'dynamic_credentials',
  'console_users',
  'console_user_logins',
  'routes',
  'route_blocks',
  'replication',
  'replication_policies',
  'replication_status',
  'replication_generations',
  'protection_groups',
  'failover_policies',
  'failover_events',
  'diff',
  'diff_report',
  'storage_tests',
  'system_settings',
] as const;

export const AUDIT_ACTION_FILTERS = [
  'create',
  'update',
  'delete',
  'set',
  'rename',
  'pause',
  'resume',
  'start',
  'restart',
  'fix',
  'enable',
  'disable',
  'block',
  'unblock',
  'rotate',
  'archive',
] as const;

// plural table names and older singular names share one label key
const FAMILY_ALIASES: Record<string, string> = {
  storages: 'storage',
  s3_identities: 's3_identity',
  dynamic_credentials: 'dynamic_credential',
  dynamic_credentials_salt: 'dynamic_credential_salt',
  proxy_accounts: 'proxy_account',
  access_policies: 'access_policy',
  console_users: 'console_user',
  console_user_logins: 'console_user_login',
  routes: 'route',
  route_blocks: 'route_block',
  replication_policies: 'replication_policy',
  replication_generations: 'replication_generation',
  failover_policies: 'failover_policy',
  failover_events: 'failover_event',
  protection_groups: 'protection_group',
  storage_tests: 'storage_test',
  diff_reports: 'diff_report',
};

const INTENT_SUFFIX = '_intent';

/** Label key suffix of a raw family ("storages" -> "storage"). */
export function familyKey(raw: string): string {
  const key = (raw || '').trim().toLowerCase();

  return FAMILY_ALIASES[key] ?? key;
}

/** "PAUSE_INTENT" -> { base: "pause", intent: true } */
export function parseAction(raw: string): { base: string; intent: boolean } {
  const key = (raw || '').trim().toLowerCase();
  const intent = key.endsWith(INTENT_SUFFIX);

  return { base: intent ? key.slice(0, -INTENT_SUFFIX.length) : key, intent };
}

/** Last resort for unknown values: "dynamic_credential_salt" -> "Dynamic credential salt". */
export function humanize(raw: string): string {
  const text = (raw || '').trim().replace(/[_-]+/g, ' ').toLowerCase();

  return text.charAt(0).toUpperCase() + text.slice(1);
}

export function actionTone(action: string): StatusTone {
  const { base, intent } = parseAction(action);

  if (intent) return 'neutral';

  switch (base) {
    case 'create':

    case 'unblock':

    case 'resume':

    case 'enable':
      return 'success';

    case 'update':

    case 'set':

    case 'rename':

    case 'start':

    case 'restart':

    case 'fix':

    case 'generation':
      return 'info';

    case 'delete':

    case 'block':
      return 'danger';

    case 'rotate':

    case 'archive':

    case 'pause':

    case 'disable':
      return 'warning';

    default:
      return 'neutral';
  }
}

/** The entry carries a before or after snapshot, so a diff can be shown. */
export function hasChanges(entry: AuditLogEntry): boolean {
  return Boolean(entry.beforeJson?.trim() || entry.afterJson?.trim());
}
