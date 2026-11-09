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
  import DashboardKpiCard from './DashboardKpiCard.vue';
  import i18nDashboard from './i18nDashboard';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import type {
    ProxyReporting,
    ReplicationBreakdown,
    StorageHealthRow,
    TrafficSummary,
  } from '@/utils/helpers/DashboardHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import { RouteName } from '@/utils/types/router';

  type Tone = 'success' | 'warning' | 'error';

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
    const tone: Tone | undefined = down > 0 ? 'error' : undefined;

    return {
      value: `${number(healthy)} / ${number(rows.length)}`,
      hint:
        unknown > 0
          ? t('kpiStoragesHint', { n: number(unknown) })
          : t('kpiStoragesAllKnown'),
      tone,
    };
  });

  const replicationsCard = computed(() => {
    const r = props.replications;

    if (!r) return { value: none.value, hint: '', tone: undefined };

    return {
      value: number(r.total),
      hint: t('kpiReplicationsHint', {
        active: number(r.active),
        paused: number(r.paused),
      }),
      tone: (r.paused > 0 ? 'warning' : undefined) as Tone | undefined,
    };
  });

  const lagCard = computed(() => {
    const r = props.replications;

    if (!r) return { value: none.value, hint: '', tone: undefined };

    return {
      value: number(r.totalLag),
      hint: r.totalLag > 0 ? t('kpiLagHint') : t('kpiLagNone'),
      tone: (r.totalLag > 0 ? 'warning' : undefined) as Tone | undefined,
    };
  });

  const requestsCard = computed(() => {
    const tr = props.traffic;

    if (!tr) return { label: t('kpiRequests'), value: none.value, hint: '' };

    if (tr.requestsPerSecond !== null) {
      return {
        label: t('kpiRequests'),
        value: TrafficStatsHelper.formatRate(tr.requestsPerSecond),
        hint: t('kpiRequestsTotalHint', { n: number(tr.requestsTotal) }),
      };
    }

    return {
      label: t('kpiRequestsTotalLabel'),
      value: number(tr.requestsTotal),
      hint: t('kpiRequestsWarmingUp'),
    };
  });

  const errorsCard = computed(() => {
    const tr = props.traffic;

    if (!tr) return { value: none.value, hint: '', tone: undefined };

    return {
      value: percent(tr.errorPercent5xx),
      hint: tr.isErrorPercentCurrent
        ? t('kpiErrorsCurrent')
        : t('kpiErrorsSinceStart'),
      tone: (tr.errorPercent5xx > 0 ? 'error' : undefined) as Tone | undefined,
    };
  });

  const proxiesCard = computed(() => {
    const p = props.proxies;

    if (!p) return { value: none.value, hint: '', tone: undefined };

    return {
      value: `${number(p.reporting)} / ${number(p.total)}`,
      hint: t('kpiProxiesHint'),
      tone: (p.reporting < p.total ? 'warning' : undefined) as Tone | undefined,
    };
  });
</script>

<template>
  <div class="dashboard-kpis">
    <div class="dashboard-kpis__grid">
      <DashboardKpiCard
        :label="t('kpiStorages')"
        :value="storagesCard.value"
        :hint="storagesCard.hint"
        :tone="storagesCard.tone"
        :to="{ name: RouteName.TUTTI_HEALTH }"
      />
      <DashboardKpiCard
        :label="t('kpiReplications')"
        :value="replicationsCard.value"
        :hint="replicationsCard.hint"
        :tone="replicationsCard.tone"
        :to="{ name: RouteName.CHORUS_REPLICATION }"
      />
      <DashboardKpiCard
        :label="t('kpiLag')"
        :value="lagCard.value"
        :hint="lagCard.hint"
        :tone="lagCard.tone"
        :to="{ name: RouteName.CHORUS_REPLICATION }"
      />
      <DashboardKpiCard
        :label="requestsCard.label"
        :value="requestsCard.value"
        :hint="requestsCard.hint"
        :to="{ name: RouteName.CHORUS_MONITORING }"
      />
      <DashboardKpiCard
        :label="t('kpiErrors')"
        :value="errorsCard.value"
        :hint="errorsCard.hint"
        :tone="errorsCard.tone"
        :to="{ name: RouteName.CHORUS_MONITORING }"
      />
      <DashboardKpiCard
        :label="t('kpiProxies')"
        :value="proxiesCard.value"
        :hint="proxiesCard.hint"
        :tone="proxiesCard.tone"
        :to="{ name: RouteName.TUTTI_HEALTH }"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .dashboard-kpis {
    container-type: inline-size;

    &__grid {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 12px;

      @container (min-width: 520px) {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }

      @container (min-width: 760px) {
        grid-template-columns: repeat(3, minmax(0, 1fr));
      }

      @container (min-width: 1100px) {
        grid-template-columns: repeat(6, minmax(0, 1fr));
      }
    }
  }
</style>
