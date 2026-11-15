<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  /** Enter the user's own key for one storage. The secret lives only here. */
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NFormItem, NInput, NModal } from 'naive-ui';
  import { Callout } from '@/components/kit';
  import messages from '@/components/tutti/users/i18nUsers';

  const props = defineProps<{
    /** storage being edited; null keeps the dialog closed */
    storage: string | null;
    busy: boolean;
    error: string;
  }>();
  const emit = defineEmits<{
    (e: 'save', accessKeyId: string, secret: string): void;
    (e: 'close'): void;
  }>();
  const { t } = useI18n({ messages });
  const accessKey = ref('');
  /** typed secret: memory only, cleared on close */
  const secretKey = ref('');

  watch(
    () => props.storage,
    () => {
      accessKey.value = '';
      secretKey.value = '';
    },
  );
</script>

<template>
  <NModal
    :show="storage !== null"
    preset="card"
    :title="t('credentialsFor', { storage: storage ?? '' })"
    style="width: 560px; max-width: 94vw"
    @update:show="!$event && emit('close')"
  >
    <form
      class="credential-modal"
      @submit.prevent="
        accessKey && secretKey && emit('save', accessKey, secretKey)
      "
    >
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <NFormItem :label="t('accessKeyId')">
        <NInput
          v-model:value="accessKey"
          :input-props="{ autocomplete: 'off' }"
        />
      </NFormItem>
      <NFormItem :label="t('secret')">
        <NInput
          v-model:value="secretKey"
          type="password"
          :input-props="{ autocomplete: 'new-password' }"
        />
      </NFormItem>
      <NButton
        type="primary"
        attr-type="submit"
        :loading="busy"
        :disabled="!accessKey || !secretKey"
        >{{ t('verifyAndSave') }}</NButton
      >
    </form>
  </NModal>
</template>

<style scoped lang="scss">
  .credential-modal {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);
  }
</style>
