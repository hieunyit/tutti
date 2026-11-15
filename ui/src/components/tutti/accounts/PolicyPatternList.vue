<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Removable bucket patterns plus one input that adds a pattern on Enter. */
  import { NButton, NInput, NTag } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nPolicy from './i18nPolicy';

  defineProps<{
    patterns: string[];
    /** current text of the add input (v-model) */
    modelValue: string;
    /** shown under the list when no pattern was added */
    emptyText?: string;
    addDisabled?: boolean;
  }>();
  const emit = defineEmits<{
    (e: 'update:modelValue', value: string): void;
    (e: 'add'): void;
    (e: 'remove', pattern: string): void;
  }>();
  const { t } = useI18n({ messages: i18nPolicy });
</script>

<template>
  <div class="pattern-list">
    <p
      v-if="patterns.length === 0 && emptyText"
      class="pattern-list__empty"
    >
      {{ emptyText }}
    </p>
    <ul
      v-if="patterns.length > 0"
      class="pattern-list__items"
    >
      <li
        v-for="pattern in patterns"
        :key="pattern"
      >
        <NTag round>{{ pattern }}</NTag>
        <NButton
          size="tiny"
          quaternary
          :aria-label="t('removePattern', { pattern })"
          @click="emit('remove', pattern)"
          >✕</NButton
        >
      </li>
    </ul>
    <div class="pattern-list__add">
      <NInput
        :value="modelValue"
        :placeholder="t('patternPlaceholder')"
        :aria-label="t('addPatternLabel')"
        @update:value="emit('update:modelValue', $event)"
        @keydown.enter.prevent="emit('add')"
      />
      <NButton
        :disabled="addDisabled ?? modelValue.trim() === ''"
        @click="emit('add')"
        >{{ t('addPattern') }}</NButton
      >
    </div>
  </div>
</template>

<style scoped lang="scss">
  .pattern-list {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);
    width: 100%;

    &__empty {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__items {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      margin: 0;
      padding: 0;
      list-style: none;

      li {
        display: inline-flex;
        align-items: center;
        gap: var(--tt-space-1);
      }
    }

    &__add {
      display: flex;
      gap: var(--tt-space-2);
    }
  }
</style>
