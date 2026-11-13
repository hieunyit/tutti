<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { NButton, NModal } from 'naive-ui';
  import { CopyValue } from '@/components/kit';

  /*
   * Shows one secret once. It lives only in this prop: nothing is stored or
   * put in the URL. The dialog cannot be closed until the user ticked that
   * the value was saved.
   */
  const props = defineProps<{
    /** null keeps the dialog closed */
    value: string | null;
    title: string;
    label: string;
    closeLabel: string;
  }>();

  const emit = defineEmits<{ (e: 'close'): void }>();
  const saved = ref(false);

  watch(
    () => props.value,
    () => {
      saved.value = false;
    },
  );
</script>

<template>
  <NModal
    :show="value !== null"
    preset="card"
    :title="title"
    style="width: 520px; max-width: 94vw"
    :mask-closable="false"
    :close-on-esc="false"
    :closable="false"
  >
    <CopyValue
      v-if="value !== null"
      :value="value"
      :label="label"
      one-time
      @acknowledged="saved = $event"
    />
    <template #footer>
      <NButton
        type="primary"
        :disabled="!saved"
        @click="emit('close')"
        >{{ closeLabel }}</NButton
      >
    </template>
  </NModal>
</template>
