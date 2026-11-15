<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->
<script setup lang="ts">
  /**
   * The fields of a new routing rule: user, buckets (all or one) and what
   * happens to the requests (send to a storage, or block). Errors show after
   * blur; the page owns the submit button.
   */
  import { computed, useId } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NFormItem, NRadio, NRadioGroup, NSelect, NSpace } from 'naive-ui';
  import i18nAddRoutingPolicy from './i18nAddRoutingPolicy';
  import { useTuttiAddRoutingPolicyStore } from '@/stores/tuttiAddRoutingPolicyStore';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';

  const { t } = useI18n({ messages: i18nAddRoutingPolicy });
  const store = useTuttiAddRoutingPolicyStore();
  const {
    users,
    storages,
    selectedUser,
    selectedToStorage,
    bucketName,
    isForAllBuckets,
    isBlocked,
    availableBuckets,
    isBucketsLoading,
    validator,
  } = storeToRefs(store);
  const uid = useId();

  const userOptions = computed(() =>
    users.value.map((user) => ({ label: user, value: user })),
  );
  const bucketOptions = computed(() =>
    availableBuckets.value.map((bucket) => ({ label: bucket, value: bucket })),
  );
  const storageOptions = computed(() =>
    GeneralHelper.orderBy(storages.value, 'name', 'asc').map((storage) => ({
      label: storage.name,
      value: storage.name,
    })),
  );

  /** first failing rule of a field, once the field was touched */
  const firstError = (field: {
    $error: boolean;
    $errors: { $message: string | unknown }[];
  }) => (field.$error ? String(field.$errors[0]?.$message ?? '') : undefined);

  const userError = computed(() => firstError(validator.value.selectedUser));
  const bucketError = computed(() => firstError(validator.value.bucketName));
  const storageError = computed(() =>
    firstError(validator.value.selectedToStorage),
  );

  const bucketScope = computed({
    get: () => (isForAllBuckets.value ? 'all' : 'one'),
    set: (value: string) => {
      isForAllBuckets.value = value === 'all';

      if (value === 'all') bucketName.value = null;
    },
  });
  const requestMode = computed({
    get: () => (isBlocked.value ? 'block' : 'route'),
    set: (value: string) => {
      isBlocked.value = value === 'block';

      if (value === 'block') selectedToStorage.value = null;
    },
  });
  const storageName = computed({
    get: () => selectedToStorage.value?.name ?? null,
    set: (name: string | null) => {
      selectedToStorage.value =
        storages.value.find((storage) => storage.name === name) ?? null;
    },
  });
</script>

<template>
  <div class="rule-form">
    <NFormItem
      :label="t('userLabel')"
      :feedback="userError ?? t('userHint')"
      :validation-status="userError ? 'error' : undefined"
    >
      <NSelect
        v-model:value="selectedUser"
        :input-props="{ 'aria-label': t('userLabel') }"
        filterable
        tag
        :options="userOptions"
        @blur="validator.selectedUser.$touch()"
      />
    </NFormItem>

    <fieldset class="rule-form__group">
      <legend>{{ t('bucketsLegend') }}</legend>
      <NRadioGroup
        v-model:value="bucketScope"
        name="routing-bucket-scope"
      >
        <NSpace vertical>
          <NRadio value="all">{{ t('bucketAll') }}</NRadio>
          <NRadio value="one">{{ t('bucketOne') }}</NRadio>
        </NSpace>
      </NRadioGroup>
      <NFormItem
        v-if="!isForAllBuckets"
        :label="t('bucketLabel')"
        :feedback="bucketError ?? t('bucketHint')"
        :validation-status="bucketError ? 'error' : undefined"
        class="rule-form__nested"
      >
        <NSelect
          v-model:value="bucketName"
          :input-props="{ id: `bucket-${uid}` }"
          filterable
          tag
          clearable
          :options="bucketOptions"
          :loading="isBucketsLoading"
          @blur="validator.bucketName.$touch()"
        />
      </NFormItem>
    </fieldset>

    <fieldset class="rule-form__group">
      <legend>{{ t('requestsLegend') }}</legend>
      <NRadioGroup
        v-model:value="requestMode"
        name="routing-request-mode"
      >
        <NSpace vertical>
          <NRadio value="route">{{ t('requestsRoute') }}</NRadio>
          <NRadio value="block">{{ t('requestsBlock') }}</NRadio>
        </NSpace>
      </NRadioGroup>
      <NFormItem
        v-if="!isBlocked"
        :label="t('storageLabel')"
        :feedback="storageError"
        :validation-status="storageError ? 'error' : undefined"
        class="rule-form__nested"
      >
        <NSelect
          v-model:value="storageName"
          :options="storageOptions"
          @blur="validator.selectedToStorage.$touch()"
        />
      </NFormItem>
      <p
        v-else
        class="rule-form__hint"
      >
        {{ t('blockHint') }}
      </p>
    </fieldset>
  </div>
</template>

<style scoped lang="scss">
  .rule-form {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);
    max-width: 520px;

    &__group {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      margin: 0;
      padding: 0;
      border: 0;

      legend {
        margin-bottom: var(--tt-space-2);
        padding: 0;
        font-weight: var(--tt-weight-strong);
      }
    }

    &__nested {
      margin-top: var(--tt-space-2);
    }

    &__hint {
      margin: var(--tt-space-1) 0 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }
  }
</style>
