<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NRadio, NRadioGroup } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nPolicy from './i18nPolicy';
  import type { GrantMode } from './policyBuckets';

  defineProps<{ modelValue: GrantMode }>();
  const emit = defineEmits<{
    (e: 'update:modelValue', value: GrantMode): void;
  }>();
  const { t } = useI18n({ messages: i18nPolicy });
  const modes: { value: GrantMode; key: string }[] = [
    { value: 'all', key: 'All' },
    { value: 'groups', key: 'Groups' },
    { value: 'custom', key: 'Custom' },
  ];
</script>

<template>
  <NRadioGroup
    :value="modelValue"
    name="policy-grant-mode"
    class="policy-radios"
    @update:value="emit('update:modelValue', $event)"
  >
    <NRadio
      v-for="mode in modes"
      :key="mode.value"
      :value="mode.value"
    >
      <strong>{{ t(`grantMode${mode.key}`) }}</strong>
      <span class="policy-radios__hint">{{
        t(`grantMode${mode.key}Hint`)
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
