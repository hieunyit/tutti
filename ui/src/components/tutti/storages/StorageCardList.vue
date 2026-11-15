<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import StorageStatusCell from './StorageStatusCell.vue';
  import i18nStorages from './i18nStorages';
  import { storageNotes } from './storageNotes';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';
  import { RouteName } from '@/utils/types/router';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

  defineProps<{
    rows: StorageRow[];
    runtime: (name: string) => RuntimeStorage | undefined;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();
</script>

<template>
  <ul class="storage-cards">
    <li
      v-for="row in rows"
      :key="row.name"
      class="storage-cards__item"
    >
      <RouterLink
        class="storage-cards__name"
        :to="{
          name: RouteName.TUTTI_STORAGE_DETAILS,
          params: { storageName: row.name },
        }"
        >{{ row.name }}</RouterLink
      >
      <small class="storage-cards__address">{{ row.address }}</small>
      <StorageStatusCell
        :status="row.health.status"
        :notes="storageNotes(runtime(row.name), row)"
      />
      <small class="storage-cards__usage">{{
        t('usageLine', {
          users: number(row.users.count),
          replications: number(
            row.replication.sourceOf + row.replication.targetOf,
          ),
          requests: number(row.requestsPerSecond),
        })
      }}</small>
    </li>
  </ul>
</template>

<style scoped lang="scss">
  .storage-cards {
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
      gap: var(--tt-space-1);
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
    }

    &__name {
      color: var(--tt-accent);
      font-weight: var(--tt-weight-strong);
      text-decoration: none;
    }

    &__address,
    &__usage {
      max-width: 100%;
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
      overflow-wrap: anywhere;
    }
  }
</style>
