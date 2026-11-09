<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import Callout from '@/components/ui/Callout.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { TuttiService } from '@/services/TuttiService';
  import { ProxiesHelper } from '@/utils/helpers/ProxiesHelper';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';
  import type { TuttiProxyStatus } from '@/utils/types/tutti';
  const { t } = useI18n({ messages });
  const proxies = ref<TuttiProxyStatus[]>([]);
  const error = ref(false);
  const busy = ref(false);
  const now = ref(Date.now());
  let timer: ReturnType<typeof setInterval> | undefined;
  let controller: AbortController | null = null;
  const drift = computed(() =>
    ProxiesHelper.settingsDrift(proxies.value, now.value),
  );
  const outliers = computed(() => ProxiesHelper.outliers(drift.value));

  async function load() {
    if (controller) return;

    const request = new AbortController();

    controller = request;
    busy.value = true;

    try {
      const result = await TuttiService.getProxies(request.signal);

      if (controller !== request) return;

      proxies.value = result.proxies ?? [];
      error.value = false;
      now.value = Date.now();
    } catch {
      if (controller === request) error.value = true;
    } finally {
      if (controller === request) {
        controller = null;
        busy.value = false;
      }
    }
  }
  onMounted(() => {
    load();
    timer = setInterval(load, 2000);
  });
  onBeforeUnmount(() => {
    clearInterval(timer);
    controller?.abort();
    controller = null;
  });
  const number = (value?: number) =>
    value === undefined
      ? '–'
      : value.toLocaleString(undefined, { maximumFractionDigits: 2 });
</script>

<template>
  <div class="phase-d">
    <PageHeader
      :title="t('proxies')"
      :description="t('proxiesHint')"
      ><template #actions
        ><NButton
          :loading="busy"
          @click="load"
          >{{ t('refresh') }}</NButton
        ></template
      ></PageHeader
    ><Callout
      v-if="drift.length"
      tone="warning"
      >{{ t('drift') }}
      <div
        v-for="item in drift"
        :key="item.field"
        class="phase-d__muted"
      >
        {{
          item.groups
            .map((group) => `${group.proxies.join(', ')}: ${group.value}`)
            .join(' · ')
        }}
      </div></Callout
    ><Callout
      v-if="error"
      tone="danger"
      >{{ t('error') }}</Callout
    ><SectionCard
      :title="t('proxies')"
      flush
      ><table
        v-if="proxies.length"
        class="phase-d__table"
      >
        <thead>
          <tr>
            <th>{{ t('proxies') }}</th>
            <th>{{ t('status') }}</th>
            <th>{{ t('uptime') }}</th>
            <th>{{ t('requests') }}</th>
            <th>{{ t('p95') }}</th>
            <th>{{ t('errors') }}</th>
            <th>{{ t('storages') }}</th>
            <th>{{ t('failover') }}</th>
            <th>{{ t('sync') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="proxy in proxies"
            :key="proxy.appId"
          >
            <td>
              <strong>{{ proxy.appId }}</strong>
              <div class="phase-d__muted">{{ proxy.address }}</div>
              <StatusTag
                v-if="outliers.has(proxy.appId)"
                variant="warning"
                >{{ t('settings') }}</StatusTag
              >
            </td>
            <td>
              <StatusTag
                :variant="
                  DashboardHelper.isProxyReporting(proxy, now)
                    ? 'success'
                    : 'warning'
                "
                >{{
                  t(
                    DashboardHelper.isProxyReporting(proxy, now)
                      ? 'reporting'
                      : 'stale',
                  )
                }}</StatusTag
              >
              <div class="phase-d__muted">
                {{ formatRelative(proxy.updatedAt, undefined, now) }}
              </div>
            </td>
            <td>{{ ProxiesHelper.uptime(proxy.startedAt, now) ?? '–' }}</td>
            <td>{{ number(proxy.rates?.requestsPerSec) }}</td>
            <td>{{ number(proxy.rates?.latencyP95Ms) }} ms</td>
            <td>{{ number(proxy.rates?.status5xxPerSec) }}</td>
            <td>
              {{ ProxiesHelper.storageAvailability(proxy).available }} /
              {{ ProxiesHelper.storageAvailability(proxy).total }}
            </td>
            <td>
              <StatusTag
                :variant="
                  proxy.settings?.failover?.enabled ? 'success' : 'neutral'
                "
                >{{
                  t(proxy.settings?.failover?.enabled ? 'on' : 'off')
                }}</StatusTag
              >
              <div class="phase-d__muted">
                {{ proxy.settings?.failover?.after }}
              </div>
            </td>
            <td>
              <StatusTag
                :variant="
                  proxy.settings?.replication?.sync?.enabled
                    ? 'success'
                    : 'neutral'
                "
                >{{
                  t(proxy.settings?.replication?.sync?.enabled ? 'on' : 'off')
                }}</StatusTag
              >
              <div
                v-if="proxy.settings?.replication?.sync?.enabled"
                class="phase-d__muted"
              >
                {{
                  t('quorum', {
                    value:
                      proxy.settings.replication.sync.writeQuorum || t('all'),
                  })
                }}
                · {{ proxy.settings.replication.sync.timeout }}
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState
        v-else-if="!busy"
        :text="t('noProxies')" /></SectionCard
    ><Callout>{{ t('settingsHint') }}</Callout>
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
