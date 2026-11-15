<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import StorageHealthTab from './StorageHealthTab.vue';
  import i18nStorages from './i18nStorages';
  import { lacksAccount } from './storageNotes';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { Callout, SectionCard } from '@/components/kit';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';
  import type {
    TuttiProxyStatus,
    TuttiStorageHealth,
  } from '@/utils/types/tutti';

  const props = defineProps<{
    row: StorageRow;
    storage: RuntimeStorage;
    proxies: {
      proxy: TuttiProxyStatus;
      health?: TuttiStorageHealth;
      fresh: boolean;
    }[];
    now: number;
  }>();
  const emit = defineEmits<{ (e: 'open-settings'): void }>();
  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();
  const replications = () =>
    props.row.replication.sourceOf + props.row.replication.targetOf;
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
    <Callout
      v-if="lacksAccount(storage)"
      tone="warning"
      :title="t('accountMissingTitle')"
    >
      {{ t('accountMissingBody') }}
      <template
        v-if="storage.source === 'managed'"
        #actions
      >
        <WriteGate>
          <NButton
            size="small"
            @click="emit('open-settings')"
            >{{ t('setAccount') }}</NButton
          >
        </WriteGate>
      </template>
    </Callout>
    <dl class="storage-overview__facts">
      <div>
        <dt>{{ t('connectionAccount') }}</dt>
        <dd>
          <code v-if="!lacksAccount(storage)">{{
            storage.connectionAccount?.accessKeyId
          }}</code>
          <template v-else>–</template>
        </dd>
      </div>
      <div>
        <dt>{{ t('requests') }}</dt>
        <dd>{{ number(row.requestsPerSecond) }}</dd>
      </div>
      <div>
        <dt>{{ t('users') }}</dt>
        <dd>{{ number(row.users.count) }}</dd>
      </div>
      <div>
        <dt>{{ t('replication') }}</dt>
        <dd>
          {{
            t('replicationSplit', {
              total: number(replications()),
              source: number(row.replication.sourceOf),
              destination: number(row.replication.targetOf),
            })
          }}
        </dd>
      </div>
      <div v-if="row.failover">
        <dt>{{ t('failover') }}</dt>
        <dd>→ {{ row.failover.target }}</dd>
      </div>
      <div v-if="row.openFailoverEvents > 0">
        <dt>{{ t('events') }}</dt>
        <dd>{{ number(row.openFailoverEvents) }}</dd>
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
    </dl>
    <SectionCard
      variant="section"
      :title="t('healthByProxy')"
    >
      <StorageHealthTab
        :rows="proxies"
        :now="now"
      />
    </SectionCard>
    <details class="storage-overview__details">
      <summary>{{ t('details') }}</summary>
      <dl class="storage-overview__facts">
        <div>
          <dt>{{ t('breaker') }}</dt>
          <dd>{{ t(row.breaker.state) }}</dd>
        </div>
        <div>
          <dt>{{ t('failureCount') }}</dt>
          <dd>{{ number(row.breaker.failures) }}</dd>
        </div>
        <div>
          <dt>{{ t('origin') }}</dt>
          <dd>
            {{ storage.source === 'managed' ? t('managed') : t('config') }}
          </dd>
        </div>
      </dl>
    </details>
  </div>
</template>

<style scoped lang="scss">
  .storage-overview {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: var(--tt-space-3) var(--tt-space-4);
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

    &__details {
      color: var(--tt-text-2);
      font-size: var(--tt-text-sm);

      summary {
        cursor: pointer;
        margin-bottom: var(--tt-space-3);
      }

      summary:focus-visible {
        outline: 2px solid var(--tt-accent);
        outline-offset: 2px;
      }
    }
  }
</style>
