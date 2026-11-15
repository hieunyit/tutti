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
  import { NButton, NFormItem, NSelect, type DataTableColumns } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink, useRouter } from 'vue-router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    DataTable,
    EmptyBlock,
    PageShell,
    StatusBadge,
    TableToolbar,
    type FilterChip,
    type PageState,
  } from '@/components/kit';
  import AutomaticFailoverControl from '@/components/tutti/protection/AutomaticFailoverControl.vue';
  import i18nProtection from '@/components/tutti/protection/i18nProtection';
  import ProtectionGroupCards from '@/components/tutti/protection/ProtectionGroupCards.vue';
  import ProtectionGroupWizard from '@/components/tutti/protection/ProtectionGroupWizard.vue';
  import {
    matchesQuery,
    toProtectionRow,
    type ProtectionRow,
  } from '@/components/tutti/protection/protectionRows';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nProtection });
  const router = useRouter();
  const narrow = useNarrowScreen();
  const store = useTuttiProtectionStore();
  const { isLoading, isLoaded, hasError, groups, statuses } =
    storeToRefs(store);

  const showCreate = ref(false);
  const query = ref('');
  const health = ref<'all' | 'ready' | 'attention'>('all');

  onBeforeMount(() => store.startPolling());
  onBeforeUnmount(() => store.stopPolling());

  const state = computed<PageState>(() => {
    if (!isLoaded.value) return hasError.value ? 'error' : 'loading';

    return groups.value.length === 0 ? 'empty' : 'ready';
  });

  const rows = computed(() =>
    groups.value.map((group) =>
      toProtectionRow(group, statuses.value[group.id], t),
    ),
  );
  const filtered = computed(() =>
    rows.value.filter(
      (row) =>
        matchesQuery(row, query.value) &&
        (health.value === 'all' ||
          (health.value === 'ready') === (row.display === 'ready')),
    ),
  );
  const healthOptions = computed(() =>
    (['all', 'ready', 'attention'] as const).map((value) => ({
      label: t(`filter.${value}`),
      value,
    })),
  );
  const chips = computed<FilterChip[]>(() =>
    health.value === 'all'
      ? []
      : [
          {
            key: 'health',
            label: `${t('colState')}: ${t(`filter.${health.value}`)}`,
          },
        ],
  );

  function clearFilters() {
    query.value = '';
    health.value = 'all';
  }

  const columns = computed<DataTableColumns<ProtectionRow>>(() => [
    {
      title: t('colName'),
      key: 'id',
      render: (row) =>
        h(
          RouterLink,
          {
            to: {
              name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
              params: { name: row.id },
            },
            class: 'tt-mono',
          },
          { default: () => row.id },
        ),
    },
    {
      title: t('colStorages'),
      key: 'storages',
      render: (row) => h('span', { class: 'tt-mono' }, row.storages),
    },
    { title: t('colScope'), key: 'covers' },
    { title: t('colCopies'), key: 'copies' },
    { title: t('colLag'), key: 'lag' },
    {
      title: t('colState'),
      key: 'state',
      render: (row) => h(StatusBadge, { tone: row.tone, label: row.state }),
    },
  ]);

  function openGroup(id: string) {
    router.push({
      name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
      params: { name: id },
    });
  }
</script>

<template>
  <PageShell
    :title="t('title')"
    :state="state"
    :error-text="t('errorText')"
    @retry="store.refresh()"
  >
    <template
      v-if="state === 'ready'"
      #actions
    >
      <WriteGate>
        <NButton
          type="primary"
          @click="showCreate = true"
        >
          {{ t('create') }}
        </NButton>
      </WriteGate>
    </template>

    <template #empty>
      <EmptyBlock
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      >
        <WriteGate>
          <NButton
            type="primary"
            @click="showCreate = true"
          >
            {{ t('create') }}
          </NButton>
        </WriteGate>
      </EmptyBlock>
      <!-- Frozen failover must stay reversible even when every group was removed. -->
      <AutomaticFailoverControl v-if="store.automaticFrozen" />
    </template>

    <div class="pg-list">
      <div class="pg-list__bar">
        <TableToolbar
          v-model:search="query"
          :search-label="t('search')"
          :search-placeholder="t('search')"
          :chips="chips"
          @remove="health = 'all'"
          @clear="clearFilters"
        >
          <NFormItem
            :label="t('colState')"
            label-placement="left"
            :show-feedback="false"
          >
            <NSelect
              v-model:value="health"
              :options="healthOptions"
              size="small"
            />
          </NFormItem>
        </TableToolbar>
        <AutomaticFailoverControl />
      </div>
      <ProtectionGroupCards
        v-if="narrow && filtered.length > 0"
        :rows="filtered"
      />
      <DataTable
        v-else
        :columns="columns"
        :rows="filtered"
        :loading="isLoading && !isLoaded"
        :row-key="(row: ProtectionRow) => row.id"
        @retry="store.refresh()"
      >
        <template #empty>
          <EmptyBlock :title="t('noMatch')">
            <NButton @click="clearFilters">{{ t('clearFilters') }}</NButton>
          </EmptyBlock>
        </template>
      </DataTable>
    </div>
  </PageShell>

  <ProtectionGroupWizard
    v-model:show="showCreate"
    @saved="openGroup"
  />
</template>

<style lang="scss" scoped>
  .pg-list {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

    &__bar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-3) var(--tt-space-5);
    }
  }
</style>
