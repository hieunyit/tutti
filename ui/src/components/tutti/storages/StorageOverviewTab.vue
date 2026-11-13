<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import { Callout, StatTile, type StatusTone } from '@/components/kit';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

  const props = defineProps<{ row: StorageRow; storage: RuntimeStorage }>();
  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();

  const healthTone = (): StatusTone =>
    props.row.health.status === 'unavailable'
      ? 'danger'
      : props.row.health.status === 'available'
        ? 'success'
        : 'neutral';
</script>

<template>
  <div class="storage-overview">
    <Callout
      v-if="row.health.status === 'unavailable'"
      tone="danger"
      :title="t('unavailable')"
      >{{ row.health.lastError || t('noHealth') }}</Callout
    >
    <Callout
      v-else-if="row.health.warning"
      tone="warning"
      :title="t('errorBadge')"
      >{{ row.health.warning }}</Callout
    >
    <div class="storage-overview__tiles">
      <StatTile
        :label="t('health')"
        :value="t(row.health.status)"
        :tone="healthTone()"
      />
      <StatTile
        :label="t('requests')"
        :value="number(row.requestsPerSecond)"
      />
      <StatTile
        :label="t('lag')"
        :value="number(row.replication.lag)"
      />
      <StatTile
        :label="t('users')"
        :value="number(row.users.count)"
      />
      <StatTile
        :label="t('rate')"
        :value="
          storage.rateLimit?.enabled
            ? `${number(storage.rateLimit.rpm)} ${t('rpm')}`
            : t('rateOff')
        "
      />
    </div>
    <dl class="storage-overview__facts">
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
    </dl>
  </div>
</template>

<style scoped lang="scss">
  .storage-overview {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__tiles {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
      gap: var(--tt-space-4);
    }

    &__facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: var(--tt-space-4);
      margin: 0;
      padding: var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);

      dt {
        color: var(--tt-text-3);
        font-size: var(--tt-text-sm);
      }

      dd {
        margin: var(--tt-space-1) 0 0;
        color: var(--tt-text);
        overflow-wrap: anywhere;
      }
    }
  }
</style>
