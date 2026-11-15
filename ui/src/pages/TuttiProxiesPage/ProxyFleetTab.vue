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
  import { useTuttiProxiesStore } from '@/stores/tuttiProxiesStore';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';
  import { ProxiesHelper } from '@/utils/helpers/ProxiesHelper';
  import type { TuttiProxyStatus } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nProxies });
  const store = useTuttiProxiesStore();
  const { proxies, configVersion, loading, error } = storeToRefs(store);
  const now = ref(Date.now());
  let clock: ReturnType<typeof setInterval> | undefined;

  const drift = computed(() =>
    ProxiesHelper.settingsDrift(proxies.value, now.value),
  );
  const outliers = computed(() => ProxiesHelper.outliers(drift.value));
  const hasGroups = computed(
    () =>
      new Set(proxies.value.map((p) => ProxiesHelper.effectiveGroup(p))).size >
      1,
  );
  const hasIssues = computed(() =>
    proxies.value.some((p) =>
      (p.storages ?? []).some((s) => !s.available || s.lastError),
    ),
  );

  const number = (value?: number) =>
    value === undefined
      ? '–'
      : value.toLocaleString(undefined, { maximumFractionDigits: 2 });

  function configCell(proxy: TuttiProxyStatus) {
    const sync = ProxiesHelper.configSyncStatus(proxy, configVersion.value);
    const badges = [];

    if (sync.state === 'applied') {
      badges.push(h(StatusBadge, { tone: 'success', label: t('inSync') }));
    } else if (sync.state === 'applying') {
      badges.push(
        h(StatusBadge, {
          tone: 'warning',
          label: t('applyingVersion', { version: sync.targetVersion }),
        }),
      );
    }

    if (proxy.configError) {
      badges.push(
        h(StatusBadge, { tone: 'danger', label: t('configRejected') }),
        h('small', proxy.configError),
      );
    }

    if (proxy.pendingRestart) {
      badges.push(
        h(StatusBadge, { tone: 'warning', label: t('pendingRestart') }),
      );
    }

    return badges.length
      ? h('div', { class: 'fleet-cell' }, badges)
      : h('span', '–');
  }

  function statusCell(proxy: TuttiProxyStatus) {
    const live = DashboardHelper.isProxyReporting(proxy, now.value);
    const s = ProxiesHelper.storageAvailability(proxy);
    const degraded = s.available < s.total;

    return h('div', { class: 'fleet-cell' }, [
      h(StatusBadge, {
        tone: !live ? 'warning' : degraded ? 'warning' : 'success',
        label: t(!live ? 'stale' : degraded ? 'degraded' : 'healthy'),
      }),
      h('small', formatRelative(proxy.updatedAt, undefined, now.value)),
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
    ...(hasGroups.value
      ? [
          {
            title: t('group'),
            key: 'group',
            render: (p: TuttiProxyStatus) => ProxiesHelper.effectiveGroup(p),
          },
        ]
      : []),
    { title: t('status'), key: 'status', render: statusCell },
    { title: t('config'), key: 'config', render: configCell },
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
  ]);

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
        >
          <NButton
            :loading="loading"
            @click="store.load()"
            >{{ t('checkAgain') }}</NButton
          >
        </EmptyBlock>
      </template>
    </DataTable>
    <template v-if="hasIssues">
      <h2 class="fleet-tab__title">{{ t('issuesTitle') }}</h2>
      <ProxyHealthCard
        v-for="proxy in proxies"
        :key="proxy.appId"
        :proxy="proxy"
      />
    </template>
  </div>
</template>

<style scoped lang="scss">
  .fleet-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

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
