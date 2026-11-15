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
  /**
   * Routing rules: which storage serves the requests of a user or bucket. The
   * title comes from the route name so the sidebar, breadcrumb, tab title and
   * H1 always agree. The header action shows only for a non-empty list (the
   * empty state carries its own); under 640px the rows become cards.
   */
  import { computed, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NButton } from 'naive-ui';
  import i18nRouting from './i18nRouting';
  import {
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    PageShell,
    TableToolbar,
    type FilterChip,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import RoutingFilters from '@/components/tutti/routing-policies/RoutingFilters.vue';
  import RoutingPolicyEditModal from '@/components/tutti/routing-policies/RoutingPolicyEditModal/RoutingPolicyEditModal.vue';
  import RoutingRuleCards from '@/components/tutti/routing-policies/RoutingRuleCards.vue';
  import { routingColumns } from '@/components/tutti/routing-policies/routingColumns';
  import {
    routingRuleKey,
    useRoutingRuleActions,
  } from '@/components/tutti/routing-policies/useRoutingRuleActions';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiRoutingPoliciesStore } from '@/stores/tuttiRoutingPoliciesStore';
  import { useDebouncedSearch } from '@/utils/composables/useDebouncedSearch';
  import { RouteName } from '@/utils/types/router';
  import {
    RoutingPolicyStatusFilter,
    type RoutingPolicy,
  } from '@/utils/types/tuttiCore';

  const { t } = useI18n({ messages: i18nRouting });
  const { t: tRoute } = useI18n({ useScope: 'global' });
  const narrow = useNarrowScreen();
  const store = useTuttiRoutingPoliciesStore();
  const {
    isLoading,
    hasError,
    hasNoData,
    computedRoutingPolicies,
    isFiltered,
    filterUsers,
    filterBuckets,
    filterStorages,
    filterStatus,
  } = storeToRefs(store);

  const { search, debounced } = useDebouncedSearch();
  const selectedKeys = ref<string[]>([]);
  const bulkDeleteOpen = ref(false);
  const editing = ref<RoutingPolicy | null>(null);

  const actions = useRoutingRuleActions((row) => {
    editing.value = row;
  });

  onBeforeMount(() => {
    store.initRoutingPoliciesPage();
  });
  onUnmounted(() => store.$reset());

  const rows = computed<RoutingPolicy[]>(() => {
    const needle = debounced.value.trim().toLowerCase();

    if (!needle) return computedRoutingPolicies.value;

    return computedRoutingPolicies.value.filter((row) =>
      [row.user, row.bucket, row.toStorage].some((part) =>
        part?.toLowerCase().includes(needle),
      ),
    );
  });
  const columns = computed(() =>
    routingColumns({ t, actionsFor: actions.actionsFor }),
  );

  const state = computed(() => {
    if (hasError.value && hasNoData.value) return 'error';

    if (isLoading.value && hasNoData.value) return 'loading';

    return hasNoData.value ? 'empty' : 'ready';
  });
  const isNarrowList = computed(() => narrow.value && rows.value.length > 0);

  const chips = computed<FilterChip[]>(() => {
    const list: FilterChip[] = [];
    const add = (key: string, label: string, value: string) =>
      list.push({ key, label: `${label}: ${value}` });

    if (filterUsers.value.length)
      add('users', t('colUser'), filterUsers.value.join(', '));

    if (filterBuckets.value.length)
      add('buckets', t('colBucket'), filterBuckets.value.join(', '));

    if (filterStorages.value.length)
      add('storages', t('colStorage'), filterStorages.value.join(', '));

    if (filterStatus.value !== null)
      add(
        'status',
        t('colStatus'),
        filterStatus.value === RoutingPolicyStatusFilter.BLOCKED
          ? t('statusBlocked')
          : t('statusAllowed'),
      );

    return list;
  });

  function removeChip(key: string) {
    if (key === 'users') filterUsers.value = [];
    else if (key === 'buckets') filterBuckets.value = [];
    else if (key === 'storages') filterStorages.value = [];
    else filterStatus.value = null;
  }

  function clearAll() {
    search.value = '';
    store.clearFilters();
  }

  const selectedRows = computed(() =>
    rows.value.filter((row) =>
      selectedKeys.value.includes(routingRuleKey(row)),
    ),
  );
  const hasBlockedSelected = computed(() =>
    selectedRows.value.some((row) => row.isBlocked),
  );

  async function confirmBulkDelete() {
    bulkDeleteOpen.value = false;
    await actions.bulkDelete(selectedRows.value);
  }
</script>

<template>
  <PageShell
    :title="tRoute(RouteName.TUTTI_ROUTING_RULES)"
    :state="state"
    :error-text="t('loadError')"
    @retry="store.initRoutingPoliciesPage()"
  >
    <template
      v-if="state === 'ready'"
      #actions
    >
      <WriteGate>
        <RouterLink :to="{ name: RouteName.TUTTI_ADD_ROUTING_POLICY }">
          <NButton
            type="primary"
            tag="span"
          >
            {{ t('add') }}
          </NButton>
        </RouterLink>
      </WriteGate>
    </template>

    <template #empty>
      <EmptyBlock
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      >
        <WriteGate>
          <RouterLink :to="{ name: RouteName.TUTTI_ADD_ROUTING_POLICY }">
            <NButton
              type="primary"
              tag="span"
            >
              {{ t('add') }}
            </NButton>
          </RouterLink>
        </WriteGate>
      </EmptyBlock>
    </template>

    <div class="routing-page">
      <TableToolbar
        v-model:search="search"
        :search-label="t('search')"
        :search-placeholder="t('searchPlaceholder')"
        :chips="chips"
        @remove="removeChip"
        @clear="clearAll"
      >
        <RoutingFilters />
      </TableToolbar>

      <RoutingRuleCards
        v-if="isNarrowList"
        :rows="rows"
        :actions-for="actions.actionsFor"
      />
      <DataTable
        v-else
        :columns="columns"
        :rows="rows"
        :loading="isLoading"
        :row-key="routingRuleKey"
        selectable
        @select="selectedKeys = $event"
      >
        <template #empty>
          <EmptyBlock :title="t('emptyFiltered')">
            <NButton
              v-if="isFiltered || search"
              @click="clearAll"
            >
              {{ t('clearFilters') }}
            </NButton>
          </EmptyBlock>
        </template>

        <template #bulk-actions>
          <WriteGate>
            <NButton
              size="small"
              @click="actions.bulkBlock(selectedRows, true)"
            >
              {{ t('block') }}
            </NButton>
            <NButton
              size="small"
              @click="actions.bulkBlock(selectedRows, false)"
            >
              {{ t('unblock') }}
            </NButton>
            <NButton
              size="small"
              type="error"
              :disabled="hasBlockedSelected"
              :title="hasBlockedSelected ? t('deleteBlocked') : undefined"
              @click="bulkDeleteOpen = true"
            >
              {{ t('deleteMany', { n: selectedRows.length }) }}
            </NButton>
          </WriteGate>
        </template>
      </DataTable>
    </div>

    <ConfirmDialog
      :open="bulkDeleteOpen"
      :title="t('deleteMany', { n: selectedRows.length })"
      danger
      :confirm-label="t('deleteMany', { n: selectedRows.length })"
      @confirm="confirmBulkDelete"
      @cancel="bulkDeleteOpen = false"
    >
      <p>{{ t('deleteManyText') }}</p>
    </ConfirmDialog>

    <ConfirmDialog
      :open="actions.pending.value !== null"
      :title="actions.pending.value?.def.label ?? ''"
      :target-name="actions.pending.value?.row.user"
      danger
      :confirm-label="actions.pending.value?.def.label ?? ''"
      @confirm="actions.confirmPending"
      @cancel="actions.cancelPending"
    >
      <p>{{ actions.pending.value?.message }}</p>
    </ConfirmDialog>

    <RoutingPolicyEditModal
      v-if="editing"
      :show="true"
      :routing-policy="editing"
      @update:show="editing = null"
    />
  </PageShell>
</template>

<style scoped lang="scss">
  .routing-page {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;
  }
</style>
