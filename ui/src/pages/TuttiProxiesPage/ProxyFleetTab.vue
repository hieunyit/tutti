<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, type DataTableColumns } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProxies from './i18nProxies';
  import {
    Callout,
    DataTable,
    EmptyBlock,
    StatusBadge,
  } from '@/components/kit';
  import ProxyHealthCard from '@/components/tutti/health/ProxyHealthCard.vue';
  import {
    PROXIES_REFRESH_INTERVAL_MS,
    useTuttiProxiesStore,
  } from '@/stores/tuttiProxiesStore';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';
  import { ProxiesHelper } from '@/utils/helpers/ProxiesHelper';
  import type { TuttiProxyStatus } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nProxies });
  const store = useTuttiProxiesStore();
  const { proxies, configVersion, loading, error, polling } =
    storeToRefs(store);
  const now = ref(Date.now());
  let clock: ReturnType<typeof setInterval> | undefined;

  const drift = computed(() =>
    ProxiesHelper.settingsDrift(proxies.value, now.value),
  );
  const outliers = computed(() => ProxiesHelper.outliers(drift.value));
  const seconds = PROXIES_REFRESH_INTERVAL_MS / 1000;

  const number = (value?: number) =>
    value === undefined
      ? '–'
      : value.toLocaleString(undefined, { maximumFractionDigits: 2 });
  const onOff = (value?: boolean) => (value ? t('on') : t('off'));

  function syncCell(proxy: TuttiProxyStatus) {
    const sync = ProxiesHelper.configSyncStatus(proxy, configVersion.value);
    const badge =
      sync.state === 'applied'
        ? h(StatusBadge, {
            tone: 'success',
            label: t('appliedVersion', { version: sync.appliedVersion }),
          })
        : sync.state === 'applying'
          ? h(StatusBadge, {
              tone: 'warning',
              label: t('applyingVersion', { version: sync.targetVersion }),
            })
          : h(StatusBadge, { tone: 'neutral', label: t('startupConfig') });

    return h('div', { class: 'fleet-cell' }, [
      badge,
      proxy.configError
        ? h('div', [
            h(StatusBadge, { tone: 'danger', label: t('configRejected') }),
            h('small', proxy.configError),
          ])
        : null,
      proxy.pendingRestart
        ? h('div', [
            h(StatusBadge, { tone: 'warning', label: t('pendingRestart') }),
            proxy.pendingRestartReasons?.length
              ? h('small', proxy.pendingRestartReasons.join(', '))
              : null,
          ])
        : null,
    ]);
  }

  const columns = computed<DataTableColumns<TuttiProxyStatus>>(() => [
    {
      title: t('proxy'),
      key: 'appId',
      render: (p) =>
        h('div', { class: 'fleet-cell' }, [
          h('strong', p.appId),
          h('small', p.address ?? ''),
          outliers.value.has(p.appId)
            ? h(StatusBadge, { tone: 'warning', label: t('outlier') })
            : null,
        ]),
    },
    {
      title: t('group'),
      key: 'group',
      render: (p) => ProxiesHelper.effectiveGroup(p),
    },
    { title: t('configSync'), key: 'sync', render: syncCell },
    {
      title: t('status'),
      key: 'status',
      render: (p) => {
        const live = DashboardHelper.isProxyReporting(p, now.value);

        return h('div', { class: 'fleet-cell' }, [
          h(StatusBadge, {
            tone: live ? 'success' : 'warning',
            label: t(live ? 'reporting' : 'stale'),
          }),
          h('small', formatRelative(p.updatedAt, undefined, now.value)),
        ]);
      },
    },
    {
      title: t('uptime'),
      key: 'uptime',
      render: (p) => ProxiesHelper.uptime(p.startedAt, now.value) ?? '–',
    },
    {
      title: t('requests'),
      key: 'rps',
      render: (p) => number(p.rates?.requestsPerSec),
    },
    {
      title: t('p95'),
      key: 'p95',
      render: (p) => `${number(p.rates?.latencyP95Ms)} ms`,
    },
    {
      title: t('errors'),
      key: 'err',
      render: (p) => number(p.rates?.status5xxPerSec),
    },
    {
      title: t('storages'),
      key: 'storages',
      render: (p) => {
        const s = ProxiesHelper.storageAvailability(p);

        return `${s.available} / ${s.total}`;
      },
    },
    {
      title: t('failover'),
      key: 'failover',
      render: (p) => onOff(p.settings?.failover?.enabled),
    },
    {
      title: t('sync'),
      key: 'syncRep',
      render: (p) => {
        const sync = p.settings?.replication?.sync;

        return sync?.enabled
          ? `${t('on')} · ${t('quorum', { value: sync.writeQuorum || t('all') })}`
          : t('off');
      },
    },
  ]);

  function togglePolling() {
    if (polling.value) store.stopPolling();
    else store.startPolling();
  }

  onMounted(() => {
    store.startPolling();
    clock = setInterval(() => (now.value = Date.now()), 5000);
  });
  onBeforeUnmount(() => {
    store.stopPolling();
    clearInterval(clock);
  });
</script>

<template>
  <div class="fleet-tab">
    <div class="fleet-tab__bar">
      <span class="fleet-tab__hint">{{
        polling ? t('autoRefresh', { seconds }) : t('paused')
      }}</span>
      <NButton
        size="small"
        @click="togglePolling"
        >{{ polling ? t('pause') : t('resume') }}</NButton
      >
      <NButton
        size="small"
        :loading="loading"
        @click="store.load()"
        >{{ t('refresh') }}</NButton
      >
    </div>
    <Callout
      v-if="drift.length"
      tone="warning"
    >
      {{ t('drift') }}
      <div
        v-for="item in drift"
        :key="item.field"
        class="fleet-tab__drift"
      >
        {{
          item.groups
            .map((group) => `${group.proxies.join(', ')}: ${group.value}`)
            .join(' · ')
        }}
      </div>
    </Callout>
    <DataTable
      :columns="columns"
      :rows="proxies"
      :loading="loading && !store.loaded"
      :error="error && proxies.length === 0 ? t('errorText') : undefined"
      :row-key="(p: TuttiProxyStatus) => p.appId"
      @retry="store.load()"
    >
      <template #empty>
        <EmptyBlock
          :title="t('noProxies')"
          :description="t('noProxiesHint')"
        />
      </template>
    </DataTable>
    <h2
      v-if="proxies.length"
      class="fleet-tab__title"
    >
      {{ t('healthTitle') }}
    </h2>
    <ProxyHealthCard
      v-for="proxy in proxies"
      :key="proxy.appId"
      :proxy="proxy"
    />
  </div>
</template>

<style scoped lang="scss">
  .fleet-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

    &__bar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: flex-end;
      gap: var(--tt-space-2);
    }

    &__hint {
      margin-right: auto;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__drift {
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__title {
      margin: var(--tt-space-2) 0 0;
      font-size: var(--tt-text-lg);
      font-weight: var(--tt-weight-strong);
    }

    :deep(.fleet-cell) {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--tt-space-1);

      small {
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
        overflow-wrap: anywhere;
      }
    }
  }
</style>
