<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  /** Search and filter controls of the audit log; applying refetches page 1. */
  import { computed, onBeforeUnmount } from 'vue';
  import { NDatePicker, NInput, NSelect } from 'naive-ui';
  import { useAuditI18n } from './useAuditI18n';
  import { FilterBar, type FilterChip } from '@/components/kit';
  import { useAuditLogStore } from '@/stores/tuttiAuditLogStore';
  import { AUDIT_LOG_ACTIONS, AUDIT_LOG_FAMILIES } from '@/utils/types/audit';

  const store = useAuditLogStore();
  const { t, familyLabel, actionLabel } = useAuditI18n();
  let timer: ReturnType<typeof setTimeout> | undefined;

  const familyOptions = computed(() =>
    AUDIT_LOG_FAMILIES.map((family) => ({
      label: `${familyLabel(family)} (${family})`,
      value: family,
    })),
  );
  const actionOptions = computed(() =>
    AUDIT_LOG_ACTIONS.map((action) => ({
      label: `${actionLabel(action)} (${action})`,
      value: action,
    })),
  );
  const limitOptions = [25, 50, 100, 200].map((n) => ({
    label: String(n),
    value: n,
  }));
  const range = computed<[string, string] | null>(() =>
    store.filters.dateFrom && store.filters.dateTo
      ? [store.filters.dateFrom, store.filters.dateTo]
      : null,
  );

  function setRange(value: [string, string] | null) {
    store.filters.dateFrom = value?.[0] ?? '';
    store.filters.dateTo = value?.[1] ?? '';
    applyNow();
  }

  // chips mirror the applied query, not the inputs still being edited
  const chips = computed<FilterChip[]>(() => {
    const list: FilterChip[] = [];

    if (store.applied.family)
      list.push({
        key: 'family',
        label: `${t('familyLabel')}: ${familyLabel(store.applied.family)}`,
      });

    if (store.applied.action)
      list.push({
        key: 'action',
        label: `${t('actionLabel')}: ${actionLabel(store.applied.action)}`,
      });

    if (store.applied.actor.trim())
      list.push({
        key: 'actor',
        label: `${t('actorLabel')}: ${store.applied.actor.trim()}`,
      });

    if (store.applied.dateFrom)
      list.push({
        key: 'date',
        label: `${store.applied.dateFrom} – ${store.applied.dateTo}`,
      });

    return list;
  });

  function applyNow() {
    clearTimeout(timer);
    void store.applyFilters();
  }

  function applyDebounced() {
    clearTimeout(timer);
    timer = setTimeout(applyNow, 400);
  }

  function setSearch(value: string) {
    store.filters.entityKey = value;
    applyDebounced();
  }

  function removeChip(key: string) {
    if (key === 'date') {
      store.filters.dateFrom = '';
      store.filters.dateTo = '';
    } else {
      (store.filters as unknown as Record<string, string>)[key] = '';
    }

    applyNow();
  }

  function clearAll() {
    clearTimeout(timer);
    void store.resetFilters();
  }

  onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <FilterBar
    :search="store.filters.entityKey"
    :search-label="t('entityKeyLabel')"
    :search-placeholder="t('entityKeyPlaceholder')"
    :chips="chips"
    @update:search="setSearch"
    @remove="removeChip"
    @clear="clearAll"
  >
    <div class="audit-filters">
      <NSelect
        v-model:value="store.filters.family"
        class="audit-filters__control"
        :aria-label="t('familyLabel')"
        :placeholder="t('familyAll')"
        :options="familyOptions"
        filterable
        clearable
        @update:value="applyNow"
      />
      <NSelect
        v-model:value="store.filters.action"
        class="audit-filters__control"
        :aria-label="t('actionLabel')"
        :placeholder="t('actionAll')"
        :options="actionOptions"
        filterable
        tag
        clearable
        @update:value="applyNow"
      />
      <NInput
        v-model:value="store.filters.actor"
        class="audit-filters__control"
        :aria-label="t('actorLabel')"
        :placeholder="t('actorPlaceholder')"
        clearable
        @update:value="applyDebounced"
        @keyup.enter="applyNow"
      />
      <NDatePicker
        :aria-label="t('dateRangeLabel')"
        :formatted-value="range"
        class="audit-filters__control"
        type="daterange"
        value-format="yyyy-MM-dd"
        @update:formatted-value="setRange"
        clearable
      />
      <NSelect
        v-model:value="store.filters.limit"
        class="audit-filters__limit"
        :aria-label="t('limitLabel')"
        :options="limitOptions"
        @update:value="applyNow"
      />
    </div>
  </FilterBar>
</template>

<style lang="scss" scoped>
  .audit-filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--tt-space-2);

    &__control {
      width: 200px;
      max-width: 100%;
    }

    &__limit {
      width: 90px;
    }
  }
</style>
