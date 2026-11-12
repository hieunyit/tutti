<!--
  - Copyright © 2026 Clyso GmbH
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
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import {
    CCollapseTransition,
    CInput,
    CSwitch,
    CFormField,
  } from '@clyso/clyso-ui-kit';
  import { NSpin, NTag } from 'naive-ui';
  import { computed } from 'vue';
  import i18nAddRoutingPolicy from '../i18nAddRoutingPolicy';
  import { useTuttiAddRoutingPolicyStore } from '@/stores/tuttiAddRoutingPolicyStore';

  const {
    isForAllBuckets,
    bucketName,
    availableBuckets,
    isBucketsLoading,
    validator,
  } = storeToRefs(useTuttiAddRoutingPolicyStore());

  const { t } = useI18n({
    messages: i18nAddRoutingPolicy,
  });

  function setForAllBuckets(value: boolean) {
    isForAllBuckets.value = value;

    if (value) {
      bucketName.value = null;
    }
  }

  function pickBucket(b: string) {
    bucketName.value = b;
    validator.value.bucketName.$touch();
  }

  const bucketNameErrorMessage = computed(() => {
    const errors = validator.value.bucketName.$errors;

    if (!errors || errors.length === 0) return '';

    return errors[0]?.$message || t('bucketNameInvalid');
  });
</script>

<template>
  <div class="bucket-selection">
    <div class="bucket-selection__header">
      <h5 class="bucket-selection__title">{{ t('bucketSelectionTitle') }}</h5>
      <p class="bucket-selection__description">
        {{ t('bucketSelectionDescription') }}
      </p>
    </div>

    <div class="bucket-selection__content">
      <div
        class="bucket-selection__switcher"
        tabindex="0"
        @click="setForAllBuckets(!isForAllBuckets)"
        @keydown.enter.prevent="setForAllBuckets(!isForAllBuckets)"
        @keydown.space.prevent="setForAllBuckets(!isForAllBuckets)"
      >
        <CSwitch
          :value="isForAllBuckets"
          size="small"
          tabindex="-1"
        />
        <span>{{ t('bucketSelectionAll') }}</span>
      </div>

      <CCollapseTransition :show="!isForAllBuckets">
        <div class="bucket-selection__input-wrapper">
          <div
            v-if="isBucketsLoading"
            class="bucket-selection__loading"
          >
            <NSpin size="small" />
            <span>{{ t('loadingBuckets') }}</span>
          </div>

          <div
            v-else-if="availableBuckets.length > 0"
            class="bucket-selection__available"
          >
            <span class="bucket-selection__available-label">
              {{ t('selectFromAvailableBuckets') }}
            </span>
            <div class="bucket-selection__tags">
              <NTag
                v-for="b in availableBuckets"
                :key="b"
                round
                checkable
                :checked="bucketName === b"
                @click="pickBucket(b)"
              >
                {{ b }}
              </NTag>
            </div>
          </div>

          <CFormField
            field-id="bucket-name-input"
            class="field-input"
            :has-error="validator.bucketName.$error"
          >
            <template #label>
              {{ t('bucketSelectionInputLabel') }}
            </template>

            <template #default="{ hasError, fieldId }">
              <CInput
                :id="fieldId"
                v-model:value="bucketName"
                :placeholder="t('bucketSelectionPlaceholder')"
                :has-error="hasError"
                @blur="validator.bucketName.$touch()"
              />
            </template>

            <template #errors>
              <template v-if="validator.bucketName.$error">
                {{ bucketNameErrorMessage }}
              </template>
            </template>
          </CFormField>
        </div>
      </CCollapseTransition>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .bucket-selection {
    &__title,
    &__description {
      margin-bottom: utils.unit(1);
    }

    &__switcher {
      display: flex;
      gap: utils.unit(2);
      cursor: pointer;
    }

    &__content {
      display: inline-grid;
      gap: utils.unit(2);
      width: 100%;
      max-width: 600px;
    }

    &__input-wrapper {
      display: flex;
      flex-direction: column;
      gap: utils.unit(2);
      margin-top: utils.unit(1);
    }

    &__loading {
      display: flex;
      align-items: center;
      gap: utils.unit(1);
      font-size: 13px;
      color: var(--n-text-color-3, #71717a);
    }

    &__available {
      display: flex;
      flex-direction: column;
      gap: utils.unit(1);
      padding: utils.unit(1.5);
      border: 1px dashed var(--n-border-color, #e0e0e6);
      border-radius: 6px;
      background: var(--n-color-embedded, rgb(0 0 0 / 2%));
    }

    &__available-label {
      color: var(--n-text-color-2, #3f3f46);
      font-weight: 500;
      font-size: 12px;
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: utils.unit(1);
    }
  }
</style>
