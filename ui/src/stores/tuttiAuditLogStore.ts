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

import { defineStore } from 'pinia';
import { computed, reactive, ref } from 'vue';
import { AuditLogService } from '@/services/AuditLogService';
import { useAuditCursor } from '@/utils/composables/useAuditCursor';
import type { AuditLogEntry } from '@/utils/types/audit';

export interface AuditFilterState {
  family: string;
  entityKey: string;
  actor: string;
  action: string;
  /** inclusive day bounds, yyyy-mm-dd, applied to the loaded entries */
  dateFrom: string;
  dateTo: string;
  limit: number;
}

function emptyFilters(): AuditFilterState {
  return {
    family: '',
    entityKey: '',
    actor: '',
    action: '',
    dateFrom: '',
    dateTo: '',
    limit: 50,
  };
}

function dayStart(day: string): number {
  return new Date(`${day}T00:00:00`).getTime();
}

function dayEnd(day: string): number {
  return new Date(`${day}T23:59:59.999`).getTime();
}

export const useAuditLogStore = defineStore('tuttiAuditLog', () => {
  const filters = reactive<AuditFilterState>(emptyFilters());
  /** filters of the last applied query: paging must not follow live edits */
  const applied = reactive<AuditFilterState>(emptyFilters());
  const backend = ref('postgres');
  const readOnly = ref(false);
  /** display-language override for the page's vi/en toggle; null = follow app */
  const localeOverride = ref<string | null>(null);

  const cursor = useAuditCursor<AuditLogEntry>(
    async (beforeId) => {
      const response = await AuditLogService.list({
        family: applied.family || undefined,
        entityKey: applied.entityKey.trim() || undefined,
        actor: applied.actor.trim() || undefined,
        action: applied.action.trim() || undefined,
        beforeId,
        limit: applied.limit,
      });
      const entries = response.entries ?? [];
      const last = entries[entries.length - 1];

      backend.value = response.backend || 'redis';
      readOnly.value = Boolean(response.readOnly);

      return {
        items: entries,
        next:
          entries.length >= applied.limit &&
          last &&
          last.id !== undefined &&
          last.id !== ''
            ? String(last.id)
            : undefined,
      };
    },
    { keyOf: (entry) => String(entry.id) },
  );

  const hasActiveFilters = computed(() =>
    Boolean(
      filters.family ||
      filters.entityKey.trim() ||
      filters.actor.trim() ||
      filters.action.trim() ||
      filters.dateFrom ||
      filters.dateTo,
    ),
  );

  /** loaded entries after the (client-side) date range */
  const entries = computed(() =>
    cursor.items.value.filter((entry) => {
      if (!filters.dateFrom && !filters.dateTo) return true;

      const at = new Date(entry.at).getTime();

      if (Number.isNaN(at)) return true;

      return (
        (!filters.dateFrom || at >= dayStart(filters.dateFrom)) &&
        (!filters.dateTo || at <= dayEnd(filters.dateTo))
      );
    }),
  );

  /** first load failed (nothing to show) vs. a later page failed */
  const loadFailed = computed(
    () => Boolean(cursor.error.value) && cursor.items.value.length === 0,
  );

  function applyFilters() {
    Object.assign(applied, filters);

    return cursor.refresh({ discardOnError: true });
  }

  function refresh() {
    Object.assign(applied, filters);

    return cursor.refresh();
  }

  function resetFilters() {
    Object.assign(filters, emptyFilters());
    Object.assign(applied, filters);

    return cursor.refresh({ discardOnError: true });
  }

  function toggleLocale(current: string) {
    localeOverride.value = current === 'vi' ? 'en' : 'vi';
  }

  return {
    filters,
    backend,
    readOnly,
    localeOverride,
    entries,
    loading: cursor.loading,
    error: cursor.error,
    loaded: cursor.loaded,
    hasOlder: cursor.hasOlder,
    hasActiveFilters,
    loadFailed,
    refresh,
    loadOlder: cursor.loadOlder,
    loadNewer: cursor.loadNewer,
    applyFilters,
    resetFilters,
    toggleLocale,
  };
});
