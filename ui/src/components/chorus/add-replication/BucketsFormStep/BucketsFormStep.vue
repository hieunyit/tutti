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
  import { storeToRefs } from 'pinia';
  import { CAlert, CCollapseTransition, CRadio } from '@clyso/clyso-ui-kit';
  import { onBeforeMount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Callout } from '@/components/kit';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
  import i18nAddReplication from '@/components/chorus/add-replication/i18nAddReplication';
  import BucketsSelectionList from '@/components/chorus/add-replication/BucketsSelectionList/BucketsSelectionList.vue';

  const {
    isForAllBuckets,
    isBucketsAlreadyRequested,
    validator,
    hasBucketsError,
    bucketsError,
    isAllBucketsDisabled,
    allBucketsDisabledReason,
  } = storeToRefs(useTuttiAddReplicationStore());
  const { initBucketsList } = useTuttiAddReplicationStore();

  const { t } = useI18n({
    messages: i18nAddReplication,
  });

  function initBucketsStep() {
    void initBucketsList(!isBucketsAlreadyRequested.value);
  }

  onBeforeMount(initBucketsStep);
</script>

<template>
  <div class="buckets-form-step">
    <Callout
      v-if="hasBucketsError && bucketsError"
      tone="danger"
      >{{ bucketsError }}</Callout
    >
    <p class="buckets-form-step__title">
      {{ t('bucketsStepTitle') }}
    </p>

    <div class="buckets-form-step__content">
      <div class="bucket-replication-selection">
        <CRadio
          :checked="!isForAllBuckets"
          :value="false"
          name="basic-demo"
          @change="isForAllBuckets = false"
        >
          {{ t('bucketsSelectOption') }}
        </CRadio>
        <CRadio
          :checked="isForAllBuckets"
          value="Definitely Maybe"
          name="basic-demo"
          :disabled="isAllBucketsDisabled"
          @change="isForAllBuckets = true"
        >
          {{ t('bucketsAllOption') }}
          <span
            v-if="isAllBucketsDisabled"
            class="bucket-replication-selection__reason"
          >
            ({{ allBucketsDisabledReason }})
          </span>
        </CRadio>
      </div>

      <CCollapseTransition :show="!isForAllBuckets">
        <BucketsSelectionList v-if="!isForAllBuckets" />
      </CCollapseTransition>

      <CCollapseTransition
        class="buckets-form-step__error"
        :show="validator.$error"
      >
        <CAlert
          type="error"
          closable
          @close="validator.$reset()"
        >
          <template #header>
            {{ t('validationErrorTitle') }}
          </template>
          {{ t('validationErrorDescription') }}
        </CAlert>
      </CCollapseTransition>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .buckets-form-step {
    &__title {
      margin-bottom: utils.unit(2);
    }

    &__error {
      margin-top: utils.unit(6);
      margin-bottom: - utils.unit(5);
      width: 450px;

      @include utils.mobile {
        width: 100%;
      }
    }
  }

  .bucket-replication-selection {
    display: inline-grid;
    gap: utils.unit(2);
    margin-bottom: utils.unit(6);
    transition: margin-bottom utils.$transition-duration;

    &:last-child {
      margin-bottom: 0;
    }

    &__reason {
      opacity: 0.7;
    }
  }
</style>
