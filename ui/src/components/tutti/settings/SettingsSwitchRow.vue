<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** A labelled on/off setting with an optional one-line hint. */
  import { NSwitch } from 'naive-ui';

  defineProps<{
    id: string;
    label: string;
    hint?: string;
    disabled?: boolean;
  }>();

  const model = defineModel<boolean>('value', { default: false });
</script>

<template>
  <div class="settings-switch">
    <div class="settings-switch__copy">
      <span
        :id="`settings-${id}-label`"
        class="settings-switch__title"
        >{{ label }}</span
      >
      <span
        v-if="hint"
        :id="`settings-${id}-hint`"
        class="settings-switch__hint"
        >{{ hint }}</span
      >
    </div>
    <NSwitch
      v-model:value="model"
      :disabled="disabled"
      :aria-labelledby="`settings-${id}-label`"
      :aria-describedby="hint ? `settings-${id}-hint` : undefined"
    />
  </div>
</template>

<style lang="scss" scoped>
  .settings-switch {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--tt-space-6);
    padding: var(--tt-space-3) 0;
    border-bottom: 1px solid var(--tt-border-subtle);

    &:last-child {
      border-bottom: 0;
    }

    &__copy {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      min-width: 0;
    }

    &__title {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text);
    }

    &__hint {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }
  }
</style>
