<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NInput, NSelect, NSkeleton } from 'naive-ui';
  import { computed, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import AddStorageDialog from '@/components/tutti/storages/AddStorageDialog.vue';
  import i18nStorages from '@/components/tutti/storages/i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import KpiCard from '@/components/ui/KpiCard.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
  import { useTuttiStoragesStore } from '@/stores/tuttiStoragesStore';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();
  const store = useTuttiStoragesStore();
  const {
    rows,
    summary,
    isLoading,
    hasError,
    managedStoragesEnabled,
    runtimeStorages,
  } = storeToRefs(store);
  const query = ref('');
  const status = ref('all');
  const showAdd = ref(false);
  const options = computed(() =>
    ['all', 'available', 'unavailable', 'unknown'].map((value) => ({
      label: t(value),
      value,
    })),
  );
  const filtered = computed(() =>
    rows.value.filter(
      (row) =>
        (status.value === 'all' || row.health.status === status.value) &&
        `${row.name} ${row.address}`
          .toLowerCase()
          .includes(query.value.toLowerCase()),
    ),
  );
  const detail = (name: string) => ({
    name: RouteName.CHORUS_STORAGE_DETAILS,
    params: { storageName: name },
  });
  const runtime = (name: string) =>
    runtimeStorages.value.find((storage) => storage.name === name);
  const source = (name: string) => runtime(name)?.source;

  onBeforeMount(store.startPolling);
  onBeforeUnmount(store.stopPolling);
</script>

<template>
  <div class="storages-page">
    <PageHeader
      :title="t('title')"
      :description="t('description')"
      ><template #actions
        ><NButton
          :loading="isLoading"
          @click="store.refresh()"
          >{{ t('refresh') }}</NButton
        ><WriteGate
          ><NButton
            type="primary"
            :disabled="!managedStoragesEnabled"
            @click="showAdd = true"
            >{{ t('add') }}</NButton
          ></WriteGate
        ></template
      ></PageHeader
    >
    <Callout
      v-if="hasError"
      tone="danger"
      >{{ t('error') }}</Callout
    >
    <WriteGate
      ><Callout v-if="!isLoading && !managedStoragesEnabled">{{
        t('disabledHint')
      }}</Callout></WriteGate
    >
    <div class="storages-page__kpis">
      <KpiCard
        :label="t('available')"
        :value="`${number(summary.available)} / ${number(summary.total)}`"
        :tone="summary.worst ? 'danger' : 'default'"
        :sub="summary.worst?.name"
      /><KpiCard
        :label="t('requests')"
        :value="number(summary.requestsPerSecond)"
      /><KpiCard
        :label="t('coverage')"
        :value="`${number(summary.usersComplete)} / ${number(summary.usersTotal)}`"
        :tone="summary.firstIncomplete ? 'warning' : 'default'"
        :sub="summary.firstIncomplete?.name"
      /><KpiCard
        :label="t('reporting')"
        :value="`${number(summary.proxiesReporting)} / ${number(summary.proxiesTotal)}`"
      />
    </div>
    <SectionCard
      :title="t('title')"
      flush
    >
      <div class="storages-page__filters">
        <NInput
          v-model:value="query"
          :placeholder="t('filter')"
          :aria-label="t('filter')"
          clearable
        /><NSelect
          v-model:value="status"
          :options="options"
          :aria-label="t('health')"
        />
      </div>
      <NSkeleton
        v-if="isLoading && !store.isLoaded"
        height="180px"
      />
      <p
        v-else-if="filtered.length === 0"
        class="storages-page__empty"
      >
        {{ t('empty') }}
      </p>
      <div
        v-else
        class="tt-table-wrap"
      >
        <table class="tt-table storages-page__table">
          <thead>
            <tr>
              <th>{{ t('name') }}</th>
              <th>{{ t('health') }}</th>
              <th>{{ t('credentialsColumn') }}</th>
              <th>{{ t('breaker') }}</th>
              <th>{{ t('requests') }}</th>
              <th>{{ t('users') }}</th>
              <th>{{ t('replication') }}</th>
              <th>{{ t('failover') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in filtered"
              :key="row.name"
            >
              <td :data-label="t('name')">
                <RouterLink
                  class="storages-page__name"
                  :to="detail(row.name)"
                  >{{ row.name }}</RouterLink
                >
                <StatusTag
                  v-if="row.isMain"
                  variant="info"
                  >{{ t('main') }}</StatusTag
                ><small>{{ row.address }}</small
                ><small>{{
                  source(row.name) === 'managed' ? t('managed') : t('config')
                }}</small>
              </td>
              <td :data-label="t('health')">
                <StatusTag
                  :variant="
                    row.health.status === 'available'
                      ? 'success'
                      : row.health.status === 'unavailable'
                        ? 'danger'
                        : 'neutral'
                  "
                  >{{ t(row.health.status) }}</StatusTag
                ><small>{{ row.health.reporting }} {{ t('reporting') }}</small
                ><StatusTag
                  v-if="row.health.warning"
                  variant="warning"
                  :title="row.health.warning"
                  >{{ t('errorBadge') }}</StatusTag
                ><small
                  v-if="row.health.status === 'available' && row.health.warning"
                  >{{ row.health.warning }}</small
                >
              </td>
              <td :data-label="t('credentialsColumn')">
                <StatusTag
                  :variant="
                    runtime(row.name)?.legacyCredentials ? 'warning' : 'success'
                  "
                  >{{
                    runtime(row.name)?.legacyCredentials
                      ? t('legacyBadge')
                      : t('accountBadge')
                  }}</StatusTag
                ><StatusTag
                  v-if="
                    StorageTestHelper.counts(runtime(row.name)?.lastTest).fail >
                      0 || runtime(row.name)?.lastTest?.ok === false
                  "
                  variant="danger"
                  >{{ t('testFailedBadge') }}</StatusTag
                ><StatusTag
                  v-else-if="runtime(row.name)?.lastTest?.hasWarnings"
                  variant="warning"
                  >{{ t('testWarnBadge') }}</StatusTag
                >
              </td>
              <td :data-label="t('breaker')">
                {{ t(row.breaker.state)
                }}<small
                  >{{ number(row.breaker.failures) }}
                  {{ t('failureCount') }}</small
                >
              </td>
              <td :data-label="t('requests')">
                {{ number(row.requestsPerSecond) }}
              </td>
              <td :data-label="t('users')">
                <RouterLink
                  :to="{ ...detail(row.name), query: { tab: 'users' } }"
                  >{{ number(row.users.count) }}</RouterLink
                >
              </td>
              <td :data-label="t('replication')">
                {{ number(row.replication.sourceOf) }} →
                {{ number(row.replication.targetOf)
                }}<small
                  >{{ t('lag') }}: {{ number(row.replication.lag) }}</small
                >
              </td>
              <td :data-label="t('failover')">
                <RouterLink
                  v-if="row.failover"
                  :to="{ name: RouteName.TUTTI_PROTECTION_GROUPS }"
                  >→ {{ row.failover.target }}</RouterLink
                ><span v-else>–</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </SectionCard>
    <AddStorageDialog
      v-model:show="showAdd"
      @created="store.refresh()"
    />
  </div>
</template>

<style scoped lang="scss">
  .storages-page {
    display: flex;
    flex-direction: column;
    gap: 20px;
    min-width: 0;

    &__kpis {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 16px;
    }

    &__filters {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      padding: 16px;

      > :first-child {
        flex: 1 1 200px;
      }

      > :last-child {
        flex: 0 1 180px;
      }
    }

    &__empty {
      padding: 16px;
      color: var(--tt-text-3);
    }

    &__name {
      font-weight: 600;
      color: var(--tt-text);
    }

    small {
      display: block;
      color: var(--tt-text-3);
      font-size: 12px;
      overflow-wrap: anywhere;
    }

    a {
      color: var(--tt-accent);
      text-decoration: none;
    }
    @media (max-width: 1000px) {
      &__kpis {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }
    @media (max-width: 760px) {
      &__table {
        display: block;

        thead {
          display: none;
        }

        tbody,
        tr {
          display: block;
        }

        tr {
          padding: 12px;
          border-bottom: 1px solid var(--tt-border);
        }

        td {
          display: grid;
          grid-template-columns: minmax(90px, 1fr) minmax(0, 2fr);
          gap: 4px 8px;
          padding: 6px 0;
          white-space: normal;
          border: 0;

          &::before {
            content: attr(data-label);
            color: var(--tt-text-3);
            font-size: 12px;
          }

          > small {
            grid-column: 2;
          }
        }
      }
    }
    @media (max-width: 440px) {
      &__kpis {
        gap: 8px;
      }
    }
  }
</style>
