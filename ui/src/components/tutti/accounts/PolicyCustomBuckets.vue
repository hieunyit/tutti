<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Custom bucket patterns: discovered buckets as one-click suggestions plus the pattern list. */
  import { NTag } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import PolicyPatternList from './PolicyPatternList.vue';
  import i18nPolicy from './i18nPolicy';

  defineProps<{
    patterns: string[];
    suggestions: string[];
    modelValue: string;
    addDisabled: boolean;
  }>();
  const emit = defineEmits<{
    (e: 'update:modelValue', value: string): void;
    (e: 'add'): void;
    (e: 'remove', pattern: string): void;
    (e: 'suggest', bucket: string): void;
  }>();
  const { t } = useI18n({ messages: i18nPolicy });
</script>

<template>
  <div class="policy-custom">
    <div
      v-if="suggestions.length > 0"
      class="policy-custom__suggestions"
    >
      <span class="policy-custom__title">{{ t('suggestionsLabel') }}</span>
      <div class="policy-custom__tags">
        <NTag
          v-for="bucket in suggestions"
          :key="bucket"
          round
          clickable
          :type="patterns.includes(bucket) ? 'primary' : 'default'"
          role="button"
          tabindex="0"
          @click="emit('suggest', bucket)"
          @keydown.enter.prevent="emit('suggest', bucket)"
          @keydown.space.prevent="emit('suggest', bucket)"
        >
          + {{ bucket }}
        </NTag>
      </div>
    </div>
    <PolicyPatternList
      :patterns="patterns"
      :model-value="modelValue"
      :empty-text="t('noPatterns')"
      :add-disabled="addDisabled"
      @update:model-value="emit('update:modelValue', $event)"
      @add="emit('add')"
      @remove="emit('remove', $event)"
    />
  </div>
</template>

<style scoped lang="scss">
  .policy-custom {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    width: 100%;

    &__suggestions {
      padding: var(--tt-space-3);
      border: 1px dashed var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
      background: var(--tt-subtle);
    }

    &__title {
      display: block;
      margin-bottom: var(--tt-space-2);
      font-size: var(--tt-text-xs);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text-2);
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }
  }
</style>
