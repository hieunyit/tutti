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
   * Step 1: source and destination, each a storage and a bucket. Errors show
   * after blur; the destination bucket follows the source bucket until it is
   * edited, because comparing a bucket with its copy is the usual case.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NFormItem, NInput, NSelect } from 'naive-ui';
  import i18nAddDiffReport from './i18nAddDiffReport';
  import { useTuttiAddDiffReportStore } from '@/stores/tuttiAddDiffReportStore';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';

  const { t } = useI18n({ messages: i18nAddDiffReport });
  const {
    storages,
    fromStorage,
    fromBucketName,
    toStorage,
    toBucketName,
    validator,
  } = storeToRefs(useTuttiAddDiffReportStore());

  const ordered = computed(() =>
    GeneralHelper.orderBy(storages.value, 'name', 'asc'),
  );
  const sourceOptions = computed(() =>
    ordered.value.map((s) => ({ label: s.name, value: s.name })),
  );
  const destinationOptions = computed(() =>
    ordered.value.map((s) => ({
      label: s.name,
      value: s.name,
      disabled: s.name === fromStorage.value?.name,
    })),
  );
  const find = (name: string | null) =>
    storages.value.find((s) => s.name === name) ?? null;

  const source = computed({
    get: () => fromStorage.value?.name ?? null,
    set: (name: string | null) => {
      fromStorage.value = find(name);
    },
  });
  const destination = computed({
    get: () => toStorage.value?.name ?? null,
    set: (name: string | null) => {
      toStorage.value = find(name);
    },
  });

  const firstError = (field: {
    $error: boolean;
    $errors: { $message: string | unknown }[];
  }) => (field.$error ? String(field.$errors[0]?.$message ?? '') : undefined);

  let followsSource = toBucketName.value === '';

  function onSourceBucketInput(value: string) {
    fromBucketName.value = value;

    if (followsSource) toBucketName.value = value;
  }

  function onDestinationBucketInput(value: string) {
    followsSource = false;
    toBucketName.value = value;
  }
</script>

<template>
  <div class="locations-step">
    <NFormItem
      :label="t('sourceStorage')"
      :feedback="firstError(validator.fromStorage)"
      :validation-status="validator.fromStorage.$error ? 'error' : undefined"
    >
      <NSelect
        v-model:value="source"
        :options="sourceOptions"
        filterable
        @blur="validator.fromStorage.$touch()"
      />
    </NFormItem>
    <NFormItem
      :label="t('destinationStorage')"
      :feedback="firstError(validator.toStorage)"
      :validation-status="validator.toStorage.$error ? 'error' : undefined"
    >
      <NSelect
        v-model:value="destination"
        :options="destinationOptions"
        filterable
        @blur="validator.toStorage.$touch()"
      />
    </NFormItem>
    <NFormItem
      :label="t('sourceBucket')"
      :feedback="firstError(validator.fromBucketName)"
      :validation-status="validator.fromBucketName.$error ? 'error' : undefined"
    >
      <NInput
        :value="fromBucketName"
        :placeholder="t('bucketPlaceholder')"
        @update:value="onSourceBucketInput"
        @blur="validator.fromBucketName.$touch()"
      />
    </NFormItem>
    <NFormItem
      :label="t('destinationBucket')"
      :feedback="firstError(validator.toBucketName)"
      :validation-status="validator.toBucketName.$error ? 'error' : undefined"
    >
      <NInput
        :value="toBucketName"
        :placeholder="t('bucketPlaceholder')"
        @update:value="onDestinationBucketInput"
        @blur="validator.toBucketName.$touch()"
      />
    </NFormItem>
  </div>
</template>

<style scoped lang="scss">
  .locations-step {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: var(--tt-space-5);
    max-width: 720px;

    @media screen and (max-width: 639px) {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
