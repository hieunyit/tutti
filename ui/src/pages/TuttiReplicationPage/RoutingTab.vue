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
  import { computed, h, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import i18nReplicationHub from './i18nReplicationHub';
  import {
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    FilterBar,
    type FilterChip,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import ChorusRowActions from '@/components/chorus/common/ChorusRowActions/ChorusRowActions.vue';
  import RowActionsConfirm from '@/components/chorus/common/ChorusRowActions/RowActionsConfirm.vue';
  import RoutingPoliciesFilters from '@/components/chorus/routing-policies/RoutingPoliciesFilters/RoutingPoliciesFilters.vue';
  import RoutingPolicyBucketCell from '@/components/chorus/routing-policies/RoutingPolicyBucketCell/RoutingPolicyBucketCell.vue';
  import RoutingPolicyStatusCell from '@/components/chorus/routing-policies/RoutingPolicyStatusCell/RoutingPolicyStatusCell.vue';
  import RoutingPolicyStorageCell from '@/components/chorus/routing-policies/RoutingPolicyStorageCell/RoutingPolicyStorageCell.vue';
  import RoutingPolicyEditModal from '@/components/chorus/routing-policies/RoutingPolicyEditModal/RoutingPolicyEditModal.vue';
  import { useTuttiRoutingPoliciesStore } from '@/stores/tuttiRoutingPoliciesStore';
  import { useRowActions } from '@/utils/composables/useRowActions';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import type { RoutingPolicy } from '@/utils/types/chorus';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nReplicationHub });
  const store = useTuttiRoutingPoliciesStore();
  const {
    isLoading,
    hasError,
    computedRoutingPolicies,
    isFiltered,
    filterUsers,
    filterBuckets,
    filterStorages,
    filterStatus,
    pageSize,
    page,
  } = storeToRefs(store);
  const { createNotification } = useTuttiNotification();

  const search = ref('');
  const selectedKeys = ref<string[]>([]);
  const bulkDeleteOpen = ref(false);
  const editing = ref<RoutingPolicy | null>(null);

  onBeforeMount(() => {
    // the kit table pages the rows itself, so the store hands over all of them
    pageSize.value = 100000;
    page.value = 1;
    store.initRoutingPoliciesPage();
  });

  onUnmounted(() => {
    store.$reset();
  });

  const rows = computed<RoutingPolicy[]>(() => {
    const needle = search.value.trim().toLowerCase();

    if (!needle) return computedRoutingPolicies.value;

    return computedRoutingPolicies.value.filter((row) =>
      [row.user, row.bucket, row.toStorage].some((part) =>
        part?.toLowerCase().includes(needle),
      ),
    );
  });

  const rowKey = (row: RoutingPolicy) => row.id;

  function report(result: { errorList: unknown[] }) {
    if (result.errorList.length) {
      createNotification({ type: 'error', title: t('actionFailed') });
    } else {
      createNotification({
        type: 'success',
        title: t('actionDone'),
        duration: 4000,
      });
    }
  }

  const { actionsFor, pending, confirmPending, cancelPending } =
    useRowActions<RoutingPolicy>(
      [
        {
          key: 'block',
          label: t('block'),
          run: async (row) =>
            report(await store.setRoutingPoliciesBlock([row], true)),
        },
        {
          key: 'unblock',
          label: t('unblock'),
          run: async (row) =>
            report(await store.setRoutingPoliciesBlock([row], false)),
        },
        {
          key: 'edit',
          label: t('edit'),
          run: async (row) => {
            editing.value = row;
          },
        },
        {
          key: 'delete',
          label: t('deletePolicy'),
          danger: true,
          run: async (row) => report(await store.deleteRoutingPolicies([row])),
          confirm: () => t('deletePolicyText'),
          disabled: (row) => row.isBlocked,
          disabledReason: () => t('actionDeleteBlocked'),
        },
      ],
      rowKey,
    );

  const columns = computed<DataTableColumns<RoutingPolicy>>(() => [
    { title: t('colUser'), key: 'user', minWidth: 140 },
    {
      title: t('colBucket'),
      key: 'bucket',
      minWidth: 200,
      render: (row) => h(RoutingPolicyBucketCell, { routingPolicy: row }),
    },
    {
      title: t('colStorage'),
      key: 'storage',
      minWidth: 160,
      render: (row) => h(RoutingPolicyStorageCell, { routingPolicy: row }),
    },
    {
      title: t('colStatus'),
      key: 'isBlocked',
      width: 150,
      render: (row) => h(RoutingPolicyStatusCell, { routingPolicy: row }),
    },
    {
      title: t('colActions'),
      key: 'actions',
      width: 300,
      render: (row) =>
        h(ChorusRowActions, {
          actions: actionsFor(row).filter(
            (action) => action.key !== (row.isBlocked ? 'block' : 'unblock'),
          ),
        }),
    },
  ]);

  const chips = computed<FilterChip[]>(() => {
    const list: FilterChip[] = [];

    if (filterUsers.value.length)
      list.push({
        key: 'users',
        label: t('chipUsers', { value: filterUsers.value.join(', ') }),
      });

    if (filterBuckets.value.length)
      list.push({
        key: 'buckets',
        label: t('chipBuckets', { value: filterBuckets.value.join(', ') }),
      });

    if (filterStorages.value.length)
      list.push({
        key: 'storages',
        label: t('chipStorages', { value: filterStorages.value.join(', ') }),
      });

    if (filterStatus.value !== null)
      list.push({ key: 'status', label: t('chipBlocked') });

    return list;
  });

  function removeChip(key: string) {
    if (key === 'users') filterUsers.value = [];
    else if (key === 'buckets') filterBuckets.value = [];
    else if (key === 'storages') filterStorages.value = [];
    else if (key === 'status') filterStatus.value = null;
  }

  function clearAll() {
    search.value = '';
    store.clearFilters();
  }

  const selectedRows = computed(() =>
    rows.value.filter((row) => selectedKeys.value.includes(rowKey(row))),
  );

  async function bulkBlock(block: boolean) {
    report(await store.setRoutingPoliciesBlock(selectedRows.value, block));
  }

  const hasBlockedSelected = computed(() =>
    selectedRows.value.some((row) => row.isBlocked),
  );

  async function bulkDelete() {
    bulkDeleteOpen.value = false;

    if (hasBlockedSelected.value) return;

    report(await store.deleteRoutingPolicies(selectedRows.value));
  }
</script>

<template>
  <div class="routing-tab">
    <FilterBar
      v-model:search="search"
      :search-label="t('search')"
      :search-placeholder="t('searchPlaceholder')"
      :chips="chips"
      @remove="removeChip"
      @clear="clearAll"
    >
      <RoutingPoliciesFilters />
    </FilterBar>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="isLoading"
      :error="hasError ? t('loadError') : undefined"
      :row-key="rowKey"
      selectable
      @select="selectedKeys = $event"
      @retry="store.initRoutingPoliciesPage()"
    >
      <template #empty>
        <EmptyBlock
          :title="
            isFiltered || search
              ? t('emptyFilteredTitle')
              : t('emptyRoutingTitle')
          "
          :description="
            isFiltered || search
              ? t('emptyFilteredText')
              : t('emptyRoutingText')
          "
        >
          <WriteGate v-if="!isFiltered && !search">
            <RouterLink :to="{ name: RouteName.CHORUS_ADD_ROUTING_POLICY }">
              <NButton
                type="primary"
                tag="span"
              >
                {{ t('addRoutingPolicy') }}
              </NButton>
            </RouterLink>
          </WriteGate>
          <NButton
            v-else
            @click="clearAll"
          >
            {{ t('clearFilters') }}
          </NButton>
        </EmptyBlock>
      </template>

      <template #bulk-actions="{ keys }">
        <WriteGate>
          <NButton
            size="small"
            @click="bulkBlock(true)"
          >
            {{ t('blockSelected', { n: keys.length }) }}
          </NButton>
          <NButton
            size="small"
            @click="bulkBlock(false)"
          >
            {{ t('unblockSelected', { n: keys.length }) }}
          </NButton>
          <NButton
            size="small"
            type="error"
            :disabled="hasBlockedSelected"
            :title="hasBlockedSelected ? t('actionDeleteBlocked') : undefined"
            @click="bulkDeleteOpen = true"
          >
            {{ t('deletePolicies', { n: keys.length }) }}
          </NButton>
        </WriteGate>
      </template>
    </DataTable>

    <ConfirmDialog
      :open="bulkDeleteOpen"
      :title="t('deletePolicies', { n: selectedRows.length })"
      danger
      :confirm-label="t('deletePolicies', { n: selectedRows.length })"
      @confirm="bulkDelete"
      @cancel="bulkDeleteOpen = false"
    >
      <p>{{ t('deletePoliciesText') }}</p>
    </ConfirmDialog>

    <RowActionsConfirm
      :pending="pending"
      :target="(row: RoutingPolicy) => row.bucket || row.user"
      @confirm="confirmPending"
      @cancel="cancelPending"
    />

    <RoutingPolicyEditModal
      v-if="editing"
      :show="true"
      :routing-policy="editing"
      @update:show="editing = null"
    />
  </div>
</template>

<style lang="scss" scoped>
  .routing-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
  }
</style>
