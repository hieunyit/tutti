<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /**
   * Storages of one proxy that need attention (unavailable, or available with
   * recent errors). Renders nothing when the proxy sees every storage healthy.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nHealth from './i18nHealth';
  import { StatusBadge } from '@/components/kit';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import type { TuttiProxyStatus } from '@/utils/types/tutti';

  const props = defineProps<{ proxy: TuttiProxyStatus }>();

  const { t } = useI18n({ messages: i18nHealth });

  const issues = computed(() =>
    (props.proxy.storages ?? []).filter(
      (storage) => !storage.available || storage.lastError,
    ),
  );
</script>

<template>
  <ul
    v-if="issues.length"
    class="proxy-health"
  >
    <li
      v-for="storage in issues"
      :key="storage.name"
      class="proxy-health__row"
    >
      <span class="proxy-health__name"
        >{{ proxy.appId }} · {{ storage.name }}</span
      >
      <StatusBadge
        :tone="storage.available ? 'warning' : 'danger'"
        :label="t(storage.available ? 'warning' : 'unavailable')"
      />
      <span
        v-if="storage.lastChange"
        class="proxy-health__meta"
        :title="formatDateTime(storage.lastChange)"
        >{{ t('since', { when: formatRelative(storage.lastChange) }) }}</span
      >
      <span
        v-if="storage.consecutiveFailures"
        class="proxy-health__meta"
        >{{ t('failures', { count: storage.consecutiveFailures }) }}</span
      >
      <span
        v-if="storage.lastError"
        class="proxy-health__error"
        >{{ storage.lastError }}</span
      >
    </li>
  </ul>
</template>

<style lang="scss" scoped>
  .proxy-health {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);
    margin: 0;
    padding: 0;
    list-style: none;

    &__row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-1) var(--tt-space-3);
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
    }

    &__name {
      font-weight: var(--tt-weight-strong);
    }

    &__meta {
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__error {
      flex-basis: 100%;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
      overflow-wrap: anywhere;
    }
  }
</style>
