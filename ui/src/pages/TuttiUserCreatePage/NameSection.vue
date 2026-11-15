<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NCheckbox, NFormItem, NInput } from 'naive-ui';
  import { Callout, FormSection } from '@/components/kit';
  import i18nUsers from '@/components/tutti/users/i18nUsers';
  import { injectUserCreate } from '@/utils/composables/useUserCreate';

  const props = defineProps<{ showErrors: boolean }>();
  const { t } = useI18n({ messages: i18nUsers });
  const ctl = injectUserCreate();
  const { form, nameProblem, credentials, credentialsLoading } = ctl;
  const touched = ref(false);

  // validated on blur (or on a submit attempt), cleared as soon as it is fixed
  const nameError = computed(() => {
    if (!touched.value && !props.showErrors) return undefined;

    if (form.value.name === '') return t('nameRequired');

    return nameProblem.value ? t('invalidName') : undefined;
  });
  const secretError = computed(() =>
    props.showErrors && credentials.value.length && !ctl.credentialsValid.value
      ? t('credentialsRequired')
      : undefined,
  );
</script>

<template>
  <FormSection :title="t('createSectionName')">
    <NFormItem
      :label="t('name')"
      :validation-status="nameError ? 'error' : undefined"
      :feedback="nameError ?? t('nameHint')"
    >
      <NInput
        v-model:value="form.name"
        :maxlength="64"
        :input-props="{ autocomplete: 'off', 'aria-required': true }"
        @blur="touched = true"
      />
    </NFormItem>
  </FormSection>
  <p
    v-if="credentialsLoading"
    role="status"
    class="name-section__status"
  >
    {{ t('loadingStorages') }}
  </p>
  <Callout
    v-if="ctl.credentialsError.value"
    tone="danger"
  >
    {{ t('storagesLoadFailed') }}
    <NButton
      :disabled="credentialsLoading"
      @click="ctl.loadCredentialRequirements"
      >{{ t('retry') }}</NButton
    >
  </Callout>
  <FormSection
    v-if="credentials.length"
    :title="t('storageCredentialsTitle')"
    :description="t('storageCredentialsHint')"
    :error="secretError"
  >
    <div
      v-for="row in credentials"
      :key="row.storage"
      class="name-section__row"
    >
      <h3 class="name-section__storage">
        {{ t('credentialsFor', { storage: row.storage }) }}
      </h3>
      <div class="name-section__fields">
        <NFormItem :label="t('accessKeyId')">
          <NInput
            v-model:value="row.accessKeyId"
            :input-props="{ autocomplete: 'off', 'aria-required': true }"
          />
        </NFormItem>
        <NFormItem :label="t('secret')">
          <NInput
            v-model:value="row.secretAccessKey"
            type="password"
            show-password-on="click"
            :input-props="{
              autocomplete: 'new-password',
              'aria-required': true,
            }"
          />
        </NFormItem>
      </div>
    </div>
    <NCheckbox v-model:checked="ctl.verifyCredentials.value">{{
      t('verifyCredentials')
    }}</NCheckbox>
  </FormSection>
</template>

<style lang="scss" scoped>
  .name-section {
    &__status {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__storage {
      margin: 0 0 var(--tt-space-1);
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
    }

    &__fields {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0 var(--tt-space-3);
    }

    @media (max-width: 639px) {
      &__fields {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
