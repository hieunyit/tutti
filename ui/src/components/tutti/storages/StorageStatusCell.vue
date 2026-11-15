<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import { healthTone, type StorageNote } from './storageNotes';
  import { StatusBadge } from '@/components/kit';

  defineProps<{ status: string; notes: StorageNote[] }>();
  const { t } = useI18n({ messages: i18nStorages });
</script>

<template>
  <div class="storage-status">
    <StatusBadge
      :tone="healthTone(status)"
      :label="t(status)"
    />
    <StatusBadge
      v-for="note in notes"
      :key="note.key"
      :tone="note.tone"
      :label="t(note.label)"
    />
  </div>
</template>

<style scoped lang="scss">
  .storage-status {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--tt-space-1) var(--tt-space-2);
  }
</style>
