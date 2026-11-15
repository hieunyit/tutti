/* * Copyright © 2026 hieunyit * * Licensed under the GNU Affero General Public
License, Version 3.0 (the "License"); * you may not use this file except in
compliance with the License. * You may obtain a copy of the License at * *
https://www.gnu.org/licenses/agpl-3.0.html * * Unless required by applicable law
or agreed to in writing, software * distributed under the License is distributed
on an "AS IS" BASIS, * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
express or implied. * See the License for the specific language governing
permissions and * limitations under the License. */

<script setup lang="ts">
  /** Synchronous-write outcomes; renders nothing until a sync write happened. */
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import { useNumberFormat } from './useNumberFormat';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';

  const { t } = useI18n({ messages: i18nMonitoring });
  const fmt = useNumberFormat();
  const store = useTuttiMonitoringStore();
  const { proxies } = storeToRefs(store);

  const sync = computed(() => {
    // depends on proxies: recomputed on every poll
    void proxies.value;

    return store.syncTotals();
  });
  const syncTotal = computed(
    () => sync.value.reached + sync.value.below + sync.value.skipped,
  );
</script>

<template>
  <SectionCard
    v-if="syncTotal > 0"
    :title="t('syncTitle')"
  >
    <dl class="monitoring-replication__sync">
      <div>
        <dt>{{ t('syncQuorumReached') }}</dt>
        <dd>{{ fmt(sync.reached) }}</dd>
      </div>
      <div>
        <dt>{{ t('syncBelowQuorum') }}</dt>
        <dd :class="{ 'monitoring-replication__bad': sync.below > 0 }">
          {{ fmt(sync.below) }}
        </dd>
      </div>
      <div>
        <dt>{{ t('syncSkipped') }}</dt>
        <dd>{{ fmt(sync.skipped) }}</dd>
      </div>
    </dl>
  </SectionCard>
</template>

<style lang="scss" scoped>
  .monitoring-replication {
    &__sync {
      display: flex;
      flex-wrap: wrap;
      gap: 12px 40px;
      margin: 0;

      dt {
        font-size: 12px;
        color: var(--tt-text-3);
      }

      dd {
        margin: 2px 0 0;
        font-size: 22px;
        line-height: 28px;
        font-weight: 700;
        font-variant-numeric: tabular-nums;
        color: var(--tt-text);
      }
    }

    &__bad {
      color: var(--tt-danger) !important;
    }
  }
</style>
