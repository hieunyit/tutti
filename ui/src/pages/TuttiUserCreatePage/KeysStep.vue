<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NCheckbox, NInput, NTag } from 'naive-ui';
  import { FormSection } from '@/components/kit';
  import messages from '@/components/tutti/users/i18nAccess';
  import { injectUserCreate } from '@/utils/composables/useUserCreate';

  const { t } = useI18n({ messages });
  const { form, effectiveBuckets } = injectUserCreate();
  const bucketsText = computed(() =>
    effectiveBuckets.value.length === 1 && effectiveBuckets.value[0] === '*'
      ? '* (all buckets)'
      : effectiveBuckets.value.join(', '),
  );
</script>

<template>
  <div class="keys-step">
    <FormSection
      :title="t('keys')"
      :description="t('stepKeysHint')"
    >
      <NCheckbox v-model:checked="form.createKey">
        {{ t('firstKey') }}
      </NCheckbox>
      <label class="keys-step__field">
        {{ t('description') }}
        <NInput v-model:value="form.description" />
      </label>
    </FormSection>
    <FormSection :title="t('summaryTitle')">
      <dl class="keys-step__summary">
        <dt>{{ t('summaryUser') }}</dt>
        <dd>
          <strong>{{ form.name }}</strong>
        </dd>
        <dt>{{ t('summaryPermission') }}</dt>
        <dd>
          <NTag
            size="small"
            type="info"
            >{{ form.permission }}</NTag
          >
        </dd>
        <dt>{{ t('summaryBuckets') }}</dt>
        <dd>
          <code>{{ bucketsText }}</code>
        </dd>
      </dl>
    </FormSection>
  </div>
</template>

<style lang="scss" scoped>
  .keys-step {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      max-width: 420px;
      margin-top: var(--tt-space-3);
      font-size: var(--tt-text-sm);
    }

    &__summary {
      display: grid;
      grid-template-columns: 140px minmax(0, 1fr);
      gap: var(--tt-space-2);
      margin: 0;
      font-size: var(--tt-text-sm);

      dt {
        color: var(--tt-text-3);
      }

      dd {
        margin: 0;
        overflow-wrap: anywhere;
      }
    }
  }
</style>
