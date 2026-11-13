<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NCheckbox, NInput } from 'naive-ui';
  import { Callout, FormSection } from '@/components/kit';
  import messages from '@/components/tutti/users/i18nAccess';
  import { injectUserCreate } from '@/utils/composables/useUserCreate';

  const { t } = useI18n({ messages });
  const ctl = injectUserCreate();
  const {
    form,
    nameProblem,
    credentials,
    credentialsLoading,
    credentialsError,
    verifyCredentials,
  } = ctl;
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
  <p
    v-if="credentialsLoading"
    role="status"
  >
    {{ t('loadingStorageAccounts') }}
  </p>
  <Callout
    v-if="credentialsError"
    tone="danger"
  >
    {{ t('storageAccountsLoadFailed') }}
    <NButton
      :disabled="credentialsLoading"
      @click="ctl.loadCredentialRequirements"
      >{{ t('retryStorageAccounts') }}</NButton
    >
  </Callout>
  <FormSection
    v-if="credentials.length"
    :title="t('storageCredentialsTitle')"
    :description="t('storageCredentialsHint')"
  >
    <div
      v-for="row in credentials"
      :key="row.storage"
      class="identity-credentials"
    >
      <h3 class="identity-credentials__title">
        {{ t('credentialsForStorage', { storage: row.storage }) }}
      </h3>
      <div class="identity-credentials__fields">
        <label
          :for="`credential-${row.storage}-access`"
          class="identity-field"
        >
          {{ t('accessKeyIdLabel') }}
          <NInput
            v-model:value="row.accessKeyId"
            :input-props="{
              id: `credential-${row.storage}-access`,
              autocomplete: 'off',
              'aria-required': true,
            }"
          />
        </label>
        <label
          :for="`credential-${row.storage}-secret`"
          class="identity-field"
        >
          {{ t('secretLabel') }}
          <NInput
            v-model:value="row.secretAccessKey"
            type="password"
            show-password-on="click"
            :input-props="{
              id: `credential-${row.storage}-secret`,
              autocomplete: 'new-password',
              'aria-required': true,
            }"
          />
        </label>
      </div>
    </div>
    <NCheckbox v-model:checked="verifyCredentials">{{
      t('verifyStorageCredentials')
    }}</NCheckbox>
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

  .identity-credentials {
    margin-bottom: var(--tt-space-4);

    &__title {
      margin: 0 0 var(--tt-space-2);
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
    }

    &__fields {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--tt-space-3);
    }
    @media (max-width: 639px) {
      &__fields {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
