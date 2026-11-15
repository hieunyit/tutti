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
   * Step 2: all buckets of the source or a selection. A bucket that is
   * already covered by a protection group or replication is shown with the
   * owner as reason and turns "All buckets" off, so the conflict is clear
   * before the server has to reject it.
   */
  import { computed, onBeforeMount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NButton, NRadio, NRadioGroup, NSpace } from 'naive-ui';
  import i18nAddReplication from '../i18nAddReplication';
  import BucketsSelectionList from '../BucketsSelectionList/BucketsSelectionList.vue';
  import { Callout } from '@/components/kit';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';

  const store = useTuttiAddReplicationStore();
  const {
    isForAllBuckets,
    selectedFromStorage,
    selectedBuckets,
    isBucketsAlreadyRequested,
    hasBucketsError,
    bucketsError,
    isAllBucketsDisabled,
    allBucketsDisabledReason,
  } = storeToRefs(store);
  const { t } = useI18n({ messages: i18nAddReplication });

  const scope = computed({
    get: () => (isForAllBuckets.value ? 'all' : 'selected'),
    set: (value: string) => {
      isForAllBuckets.value = value === 'all';
    },
  });

  function load() {
    void store.initBucketsList(!isBucketsAlreadyRequested.value);
  }

  onBeforeMount(load);
</script>

<template>
  <div class="buckets-step">
    <NRadioGroup
      v-model:value="scope"
      name="replication-bucket-scope"
      :aria-label="t('stepBuckets')"
    >
      <NSpace vertical>
        <NRadio value="selected">{{ t('bucketsSelected') }}</NRadio>
        <NRadio
          value="all"
          :disabled="isAllBucketsDisabled"
        >
          {{ t('bucketsAll', { storage: selectedFromStorage?.name }) }}
        </NRadio>
      </NSpace>
    </NRadioGroup>
    <p
      v-if="isAllBucketsDisabled"
      class="buckets-step__note"
    >
      {{ t('allDisabled', { reason: allBucketsDisabledReason }) }}
    </p>

    <Callout
      v-if="hasBucketsError"
      tone="danger"
    >
      {{ bucketsError || t('initBucketsError') }}
      <template #actions>
        <NButton
          size="small"
          @click="load"
        >
          {{ t('retry') }}
        </NButton>
      </template>
    </Callout>
    <template v-else-if="!isForAllBuckets">
      <BucketsSelectionList />
      <p
        v-if="selectedBuckets.length === 0"
        class="buckets-step__note"
      >
        {{ t('selectOne') }}
      </p>
    </template>
  </div>
</template>

<style scoped lang="scss">
  .buckets-step {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);

    &__note {
      margin: 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }
  }
</style>
