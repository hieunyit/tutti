<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton } from 'naive-ui';
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import { Callout, ConfirmDialog, SectionCard } from '@/components/kit';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

  defineProps<{ storage: RuntimeStorage; busy: boolean; error: string }>();
  const emit = defineEmits<{ (e: 'confirm'): void; (e: 'cancel'): void }>();
  const { t } = useI18n({ messages: i18nStorages });
  const open = ref(false);

  /** closed by the parent after a successful delete */
  defineExpose({
    close: () => {
      open.value = false;
    },
  });
</script>

<template>
  <SectionCard :title="t('deleteTitle')">
    <p>{{ t('deleteHint') }}</p>
    <p
      v-if="!RuntimeStorageHelper.canDelete(storage)"
      class="storage-delete__sub"
    >
      {{ t('deleteBlocked') }}
    </p>
    <NButton
      type="error"
      secondary
      :disabled="busy || !RuntimeStorageHelper.canDelete(storage)"
      @click="open = true"
      >{{ t('delete') }}</NButton
    >
    <ConfirmDialog
      :open="open"
      :title="t('deleteTitle')"
      :target-name="storage.name"
      require-typing
      danger
      :confirm-label="t('delete')"
      :loading="busy"
      @confirm="emit('confirm')"
      @cancel="
        open = false;
        emit('cancel');
      "
    >
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <p>{{ t('deleteConfirm', { name: storage.name }) }}</p>
    </ConfirmDialog>
  </SectionCard>
</template>

<style scoped lang="scss">
  .storage-delete__sub {
    color: var(--tt-text-3);
    font-size: var(--tt-text-sm);
  }
</style>
