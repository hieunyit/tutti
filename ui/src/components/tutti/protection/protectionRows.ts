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

import { EMPTY_VALUE, type StatusTone } from '@/components/kit';
import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
import type {
  ProtectionGroup,
  ProtectionGroupStatus,
} from '@/utils/types/protection';

type Translate = (key: string, values?: Record<string, unknown>) => string;

/** One list row: everything the table and the narrow cards print. */
export interface ProtectionRow {
  id: string;
  /** "hcm02 → hcm04": primary first, then the other members */
  storages: string;
  /** "12 buckets", "All buckets of alice", ... */
  covers: string;
  copies: string;
  lag: string;
  tone: StatusTone;
  /** display state key (ready, error, ...) for the status filter */
  display: string;
  state: string;
}

/** Primary first, the other members in stored order, joined with arrows. */
export function storagesText(group: ProtectionGroup): string {
  const members = group.members ?? [];
  const primary = group.primary && members.includes(group.primary);
  const ordered = primary
    ? [group.primary as string, ...members.filter((m) => m !== group.primary)]
    : members;

  return ordered.length ? ordered.join(' → ') : EMPTY_VALUE;
}

export function coversText(group: ProtectionGroup, t: Translate): string {
  const kind = ProtectionGroupHelper.scopeKind(group);

  if (kind === 'storage') return t('scopeStorage');

  if (kind === 'user') return t('scopeUser', { user: group.user });

  const count = (group.buckets ?? []).length;

  return count === 1 ? t('scopeBucketsOne') : t('scopeBuckets', { count });
}

export function toProtectionRow(
  group: ProtectionGroup,
  status: ProtectionGroupStatus | undefined,
  t: Translate,
): ProtectionRow {
  const display = ProtectionGroupHelper.displayState(group);

  return {
    id: group.id,
    storages: storagesText(group),
    covers: coversText(group, t),
    copies: t('copiesValue', {
      healthy: status?.healthyCopies ?? EMPTY_VALUE,
      wanted: ProtectionGroupHelper.wantedCopies(group),
    }),
    lag: ProtectionGroupHelper.lagText(ProtectionGroupHelper.maxLag(status)),
    tone: ProtectionGroupHelper.stateVariant(display) as StatusTone,
    display,
    state: t(`state.${display}`),
  };
}

/** Text filter over name, storages and scope (case-insensitive). */
export function matchesQuery(row: ProtectionRow, query: string): boolean {
  const needle = query.trim().toLowerCase();

  return (
    needle === '' ||
    `${row.id} ${row.storages} ${row.covers}`.toLowerCase().includes(needle)
  );
}
