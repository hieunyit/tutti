<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NRadio, NRadioGroup } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nPolicy from './i18nPolicy';
  import { TuttiAccessPermission } from '@/utils/types/tutti';

  defineProps<{ modelValue: TuttiAccessPermission }>();
  const emit = defineEmits<{
    (e: 'update:modelValue', value: TuttiAccessPermission): void;
  }>();
  const { t } = useI18n({ messages: i18nPolicy });
  const options = computed(() => [
    { value: TuttiAccessPermission.READ_WRITE, key: 'ReadWrite' },
    { value: TuttiAccessPermission.READ_ONLY, key: 'ReadOnly' },
    { value: TuttiAccessPermission.WRITE_ONLY, key: 'WriteOnly' },
  ]);
</script>

<template>
  <NRadioGroup
    :value="modelValue"
    name="policy-permission"
    class="policy-radios"
    @update:value="emit('update:modelValue', $event)"
  >
    <NRadio
      v-for="option in options"
      :key="option.value"
      :value="option.value"
    >
      <strong>{{ t(`permission${option.key}`) }}</strong>
      <span class="policy-radios__hint">{{
        t(`permission${option.key}Hint`)
      }}</span>
    </NRadio>
  </NRadioGroup>
</template>

<style lang="scss">
  .policy-radios {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);

    &__hint {
      display: block;
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }
  }
</style>
