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
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import { StatTile, type StatusTone } from '@/components/kit';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import type {
    ProxyReporting,
    ReplicationBreakdown,
    StorageHealthRow,
    TrafficSummary,
  } from '@/utils/helpers/DashboardHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    storages: StorageHealthRow[] | null;
    replications: ReplicationBreakdown | null;
    proxies: ProxyReporting | null;
    traffic: TrafficSummary | null;
  }>();

  const { t, locale } = useI18n({ messages: i18nDashboard });
  const number = useNumberFormat();

  const none = computed(() => t('kpiNoData'));
  const percent = (v: number) =>
    new Intl.NumberFormat(locale.value === 'de' ? 'de-DE' : 'en-GB', {
      style: 'percent',
      minimumFractionDigits: v < 10 ? 2 : 1,
      maximumFractionDigits: v < 10 ? 2 : 1,
    }).format(v / 100);

  const storagesCard = computed(() => {
    const rows = props.storages;

    if (!rows) return { value: none.value, hint: '', tone: undefined };

    const healthy = rows.filter((r) => r.state === 'healthy').length;
    const down = rows.filter((r) => r.state === 'unavailable').length;
    const unknown = rows.filter((r) => r.state === 'unknown').length;
    const tone: StatusTone | undefined = down > 0 ? 'danger' : undefined;

    return {
      value: `${number(healthy)} / ${number(rows.length)}`,
      hint:
        down > 0
          ? t('kpiStoragesDown', {
              names: rows
                .filter((r) => r.state === 'unavailable')
                .map((r) => r.name)
                .join(', '),
            })
          : unknown > 0
            ? t('kpiStoragesHint', { n: number(unknown) })
            : t('kpiStoragesAllKnown'),
      tone,
    };
  });

  const replicationsCard = computed(() => {
    const r = props.replications;

    if (!r) return { value: none.value, hint: '', tone: undefined };

    return {
      value: `${number(r.active)} ${t('kpiReplicationsActive')}`,
      hint: t('kpiReplicationsPausedHint', { n: number(r.paused) }),
      tone: (r.paused > 0 ? 'warning' : undefined) as StatusTone | undefined,
    };
  });

  const lagCard = computed(() => {
    const r = props.replications;

    if (!r) return { value: none.value, hint: '', tone: undefined };

    return {
      value: number(r.totalLag),
      hint: r.totalLag > 0 ? t('kpiLagHint') : t('kpiLagNone'),
      tone: (r.totalLag > 0 ? 'warning' : undefined) as StatusTone | undefined,
    };
  });

  const requestsCard = computed(() => {
    const tr = props.traffic;

    if (!tr) return { value: none.value, hint: '' };

    return {
      value: TrafficStatsHelper.formatRate(tr.requestsPerSecond),
      hint: t('kpiRequestsHint'),
    };
  });

  const errorsCard = computed(() => {
    const tr = props.traffic;

    if (!tr) return { value: none.value, hint: '', tone: undefined };

    return {
      value: percent(tr.errorPercent5xx),
      hint: t('kpiErrorsCurrent'),
      tone: (tr.errorPercent5xx >= 5
        ? 'danger'
        : tr.errorPercent5xx >= 1
          ? 'warning'
          : undefined) as StatusTone | undefined,
    };
  });

  const proxiesCard = computed(() => {
    const p = props.proxies;

    if (!p) return { value: none.value, hint: '', tone: undefined };

    return {
      value: `${number(p.reporting)} / ${number(p.total)}`,
      hint: t('kpiProxiesHint'),
      tone: (p.reporting < p.total ? 'warning' : undefined) as
        | StatusTone
        | undefined,
    };
  });
</script>

<template>
  <section
    class="dashboard-kpis"
    :aria-label="t('kpiLabel')"
  >
    <StatTile
      :label="t('kpiStorages')"
      :value="storagesCard.value"
      :hint="storagesCard.hint"
      :tone="storagesCard.tone"
      :to="{ name: RouteName.TUTTI_STORAGES }"
    />
    <StatTile
      :label="t('kpiReplications')"
      :value="replicationsCard.value"
      :hint="replicationsCard.hint"
      :tone="replicationsCard.tone"
      :to="{ name: RouteName.TUTTI_REPLICATION }"
    />
    <StatTile
      :label="t('kpiLag')"
      :value="lagCard.value"
      :hint="lagCard.hint"
      :tone="lagCard.tone"
      :to="{ name: RouteName.TUTTI_REPLICATION }"
    />
    <StatTile
      :label="t('kpiRequests')"
      :value="requestsCard.value"
      :hint="requestsCard.hint"
      :to="{ name: RouteName.TUTTI_DASHBOARD, query: { tab: 'metrics' } }"
    />
    <StatTile
      :label="t('kpiErrors')"
      :value="errorsCard.value"
      :hint="errorsCard.hint"
      :tone="errorsCard.tone"
      :to="{ name: RouteName.TUTTI_DASHBOARD, query: { tab: 'metrics' } }"
    />
    <StatTile
      :label="t('kpiProxies')"
      :value="proxiesCard.value"
      :hint="proxiesCard.hint"
      :tone="proxiesCard.tone"
      :to="{ name: RouteName.TUTTI_STORAGES }"
    />
  </section>
</template>

<style lang="scss" scoped>
  .dashboard-kpis {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 16px;
  }
</style>
