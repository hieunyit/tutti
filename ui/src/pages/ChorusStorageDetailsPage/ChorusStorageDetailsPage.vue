<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NSkeleton } from 'naive-ui';
  import { computed, onBeforeMount, onBeforeUnmount } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import BackLink from '@/components/tutti/common/BackLink.vue';
  import TabBar from '@/components/tutti/common/TabBar.vue';
  import StorageSettings from '@/components/tutti/storages/StorageSettings.vue';
  import i18nStorages from '@/components/tutti/storages/i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import KpiCard from '@/components/ui/KpiCard.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import { useTuttiStoragesStore } from '@/stores/tuttiStoragesStore';
  import { useQueryTab } from '@/utils/composables/useQueryTab';
  import { useNow } from '@/utils/composables/useNow';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import { StoragesPageHelper } from '@/utils/helpers/StoragesPageHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ storageName: string }>();
  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();
  const router = useRouter();
  const store = useTuttiStoragesStore();
  const now = useNow();
  const tab = useQueryTab(
    ['overview', 'health', 'users', 'settings'] as const,
    'overview',
  );
  const storage = computed(() =>
    store.runtimeStorages.find((item) => item.name === props.storageName),
  );
  const row = computed(() =>
    StoragesPageHelper.rows(
      {
        storages: store.storages,
        users: store.users,
        proxies: store.proxies,
        policies: store.policies,
        replications: store.replications,
        events: store.events,
      },
      now.value,
    ).find((item) => item.name === props.storageName),
  );
  const tabs = computed(() => [
    { value: 'overview', label: t('overview') },
    { value: 'health', label: t('healthByProxy') },
    {
      value: 'users',
      label: t('users'),
      count: storage.value?.users?.length ?? 0,
    },
    { value: 'settings', label: t('settings') },
  ]);
  const health = computed(() =>
    store.proxies.map((proxy) => ({
      proxy,
      health: proxy.storages?.find((item) => item.name === props.storageName),
      fresh: DashboardHelper.isProxyReporting(proxy, now.value),
    })),
  );

  onBeforeMount(store.startPolling);
  onBeforeUnmount(store.stopPolling);

  async function deleted() {
    await store.refresh();
    await router.push({ name: RouteName.CHORUS_STORAGES });
  }
</script>

<template>
  <div class="storage-detail">
    <BackLink
      :to="{ name: RouteName.CHORUS_STORAGES }"
      :label="t('backToStorages')"
    />
    <PageHeader
      :title="storageName"
      :description="
        storage
          ? `${storage.address} · ${storage.provider} · ${storage.source === 'managed' ? t('managed') : t('config')}`
          : ''
      "
      ><template #actions
        ><NButton
          :loading="store.isLoading"
          @click="store.refresh()"
          >{{ t('refresh') }}</NButton
        ></template
      ></PageHeader
    >
    <Callout
      v-if="store.hasError"
      tone="danger"
      >{{ t('error') }}</Callout
    >
    <NSkeleton
      v-if="!store.isLoaded && store.isLoading"
      height="240px"
    />
    <Callout v-else-if="!storage">{{ t('notFound') }}</Callout>
    <template v-else>
      <TabBar
        v-model="tab"
        :tabs="tabs"
        :label="storageName"
      />
      <section
        :id="`tabpanel-${tab}`"
        role="tabpanel"
        :aria-labelledby="`tab-${tab}`"
      >
        <template v-if="tab === 'overview' && row">
          <Callout
            v-if="row.health.status === 'unavailable'"
            tone="danger"
            :title="t('unavailable')"
            >{{ row.health.lastError || t('noHealth') }}</Callout
          >
          <div class="storage-detail__kpis">
            <KpiCard
              :label="t('health')"
              :value="t(row.health.status)"
              :tone="row.health.status === 'unavailable' ? 'danger' : 'default'"
            /><KpiCard
              :label="t('requests')"
              :value="number(row.requestsPerSecond)"
            /><KpiCard
              :label="t('lag')"
              :value="number(row.replication.lag)"
            /><KpiCard
              :label="t('users')"
              :value="number(row.users.count)"
            />
          </div>
          <SectionCard :title="t('replication')"
            ><dl class="storage-detail__facts">
              <div>
                <dt>{{ t('sourceOf') }}</dt>
                <dd>{{ number(row.replication.sourceOf) }}</dd>
              </div>
              <div>
                <dt>{{ t('targetOf') }}</dt>
                <dd>{{ number(row.replication.targetOf) }}</dd>
              </div>
              <div>
                <dt>{{ t('failover') }}</dt>
                <dd>{{ row.failover ? `→ ${row.failover.target}` : '–' }}</dd>
              </div>
              <div>
                <dt>{{ t('events') }}</dt>
                <dd>{{ number(row.openFailoverEvents) }}</dd>
              </div>
              <div>
                <dt>{{ t('breaker') }}</dt>
                <dd>{{ t(row.breaker.state) }}</dd>
              </div>
              <div>
                <dt>{{ t('rate') }}</dt>
                <dd>
                  {{
                    storage.rateLimit?.enabled
                      ? `${number(storage.rateLimit.rpm)} ${t('rpm')}`
                      : t('rateOff')
                  }}
                </dd>
              </div>
            </dl></SectionCard
          >
        </template>
        <SectionCard
          v-else-if="tab === 'health'"
          :title="t('healthByProxy')"
          flush
        >
          <p
            v-if="health.length === 0"
            class="storage-detail__empty"
          >
            {{ t('noHealth') }}
          </p>
          <div
            v-else
            class="tt-table-wrap"
          >
            <table class="tt-table storage-detail__table">
              <thead>
                <tr>
                  <th>{{ t('name') }}</th>
                  <th>{{ t('health') }}</th>
                  <th>{{ t('failureCount') }}</th>
                  <th>{{ t('lastError') }}</th>
                  <th>{{ t('lastChange') }}</th>
                  <th>{{ t('lastReport') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in health"
                  :key="item.proxy.appId"
                >
                  <td :data-label="t('name')">
                    {{ item.proxy.appId
                    }}<small>{{ item.proxy.address }}</small>
                  </td>
                  <td :data-label="t('health')">
                    <StatusTag
                      :variant="
                        !item.fresh || item.health?.available === undefined
                          ? 'neutral'
                          : item.health.available
                            ? 'success'
                            : 'danger'
                      "
                      >{{
                        !item.fresh || item.health?.available === undefined
                          ? t('unknown')
                          : item.health.available
                            ? t('available')
                            : t('unavailable')
                      }}</StatusTag
                    >
                  </td>
                  <td :data-label="t('failureCount')">
                    {{
                      item.fresh
                        ? number(item.health?.consecutiveFailures ?? 0)
                        : '–'
                    }}
                  </td>
                  <td :data-label="t('lastError')">
                    {{ item.health?.lastError || '–' }}
                  </td>
                  <td :data-label="t('lastChange')">
                    {{ formatDateTime(item.health?.lastChange) }}
                  </td>
                  <td :data-label="t('lastReport')">
                    {{ formatRelative(item.proxy.updatedAt, undefined, now) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </SectionCard>
        <SectionCard
          v-else-if="tab === 'users'"
          :title="t('users')"
          :link-label="t('manageUsers')"
          :link-to="{ name: RouteName.TUTTI_USERS }"
          flush
          ><p
            v-if="!storage.users?.length"
            class="storage-detail__empty"
          >
            {{ t('searchEmpty') }}
          </p>
          <div
            v-else
            class="tt-table-wrap"
          >
            <table class="tt-table storage-detail__table">
              <thead>
                <tr>
                  <th>{{ t('user') }}</th>
                  <th>{{ t('accessKey') }}</th>
                  <th>{{ t('source') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="user in storage.users"
                  :key="user.user"
                >
                  <td :data-label="t('user')">
                    <RouterLink
                      :to="{
                        name: RouteName.TUTTI_USER_DETAIL,
                        params: { name: user.user },
                      }"
                      >{{ user.user }}</RouterLink
                    >
                  </td>
                  <td :data-label="t('accessKey')">{{ user.accessKeyId }}</td>
                  <td :data-label="t('source')">
                    {{ user.source === 'config' ? t('config') : t('managed') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div></SectionCard
        >
        <StorageSettings
          v-else-if="tab === 'settings'"
          :key="storageName"
          :storage="storage"
          @saved="store.refresh()"
          @deleted="deleted"
        />
      </section>
    </template>
  </div>
</template>

<style scoped lang="scss">
  .storage-detail {
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;

    &__kpis {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 16px;
      margin: 16px 0;
    }

    &__facts {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 20px;
      margin: 0;

      dt {
        color: var(--tt-text-3);
        font-size: 13px;
      }

      dd {
        margin: 4px 0 0;
        color: var(--tt-text);
      }
    }

    &__empty {
      padding: 16px;
      color: var(--tt-text-3);
    }

    small {
      display: block;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    a {
      color: var(--tt-accent);
      text-decoration: none;
    }

    td,
    dd {
      overflow-wrap: anywhere;
    }
    @media (max-width: 800px) {
      &__kpis {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }

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
          gap: 8px;
          padding: 6px 0;
          border: 0;
          white-space: normal;

          &::before {
            content: attr(data-label);
            color: var(--tt-text-3);
            font-size: 12px;
          }

          small {
            grid-column: 2;
          }
        }
      }
    }
  }
</style>
