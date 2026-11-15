<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Access keys as cards on narrow screens. */
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import KeyRowMenu from './KeyRowMenu.vue';
  import i18nAccounts from './i18nAccounts';
  import { isDisabled, sourceLabel, type KeyRow } from './accessKeyRows';
  import { CopyValue, StatusBadge } from '@/components/kit';
  import { RouteName } from '@/utils/types/router';

  defineProps<{ rows: KeyRow[] }>();
  const emit = defineEmits<{
    (e: 'rotate', accessKeyId: string): void;
    (e: 'toggle', row: KeyRow): void;
    (e: 'delete', row: KeyRow): void;
  }>();
  const { t } = useI18n({ messages: i18nAccounts });
</script>

<template>
  <ul class="key-cards">
    <li
      v-for="row in rows"
      :key="`${row.user}:${row.accessKeyId}`"
      class="key-cards__item"
    >
      <CopyValue
        :value="row.accessKeyId"
        :label="t('columnAccessKeyId')"
      />
      <RouterLink
        :to="{
          name: RouteName.TUTTI_USER_DETAIL,
          params: { name: row.user },
        }"
        >{{ row.user }}</RouterLink
      >
      <span class="key-cards__meta">
        <StatusBadge
          :tone="isDisabled(row) ? 'neutral' : 'success'"
          :label="t(isDisabled(row) ? 'statusDisabled' : 'statusActive')"
        />
        <small>{{ sourceLabel(row.source, t) }}</small>
      </span>
      <KeyRowMenu
        :row="row"
        @rotate="emit('rotate', row.accessKeyId)"
        @toggle="emit('toggle', row)"
        @delete="emit('delete', row)"
      />
    </li>
  </ul>
</template>

<style scoped lang="scss">
  .key-cards {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;
    padding: 0;
    list-style: none;

    &__item {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--tt-space-2);
      min-width: 0;
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
    }

    &__meta {
      display: inline-flex;
      gap: var(--tt-space-2);
      align-items: center;

      small {
        font-size: var(--tt-text-xs);
        color: var(--tt-text-3);
      }
    }
  }
</style>
