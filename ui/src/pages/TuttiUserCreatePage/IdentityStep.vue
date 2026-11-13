<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NInput } from 'naive-ui';
  import { FormSection } from '@/components/kit';
  import messages from '@/components/tutti/users/i18nAccess';
  import { injectUserCreate } from '@/utils/composables/useUserCreate';

  const { t } = useI18n({ messages });
  const { form, nameProblem } = injectUserCreate();
  const touched = ref(false);

  // validate on blur, clear as soon as the value is fixed
  const error = computed(() =>
    touched.value && nameProblem.value ? t('invalidName') : undefined,
  );
</script>

<template>
  <FormSection
    :title="t('identity')"
    :description="t('nameHint')"
    :error="error"
  >
    <label class="identity-field">
      {{ t('name') }}
      <NInput
        v-model:value="form.name"
        :maxlength="64"
        :status="error ? 'error' : undefined"
        :input-props="{ autocomplete: 'off' }"
        @blur="touched = true"
      />
    </label>
  </FormSection>
</template>

<style lang="scss" scoped>
  .identity-field {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);
    max-width: 420px;
    font-size: var(--tt-text-sm);
  }
</style>
