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
  import { NFormItem } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import ReplicationsFilterByUser from '@/components/tutti/replications/ReplicationsFilterByUser/ReplicationsFilterByUser.vue';
  import ReplicationsFilterByBucket from '@/components/tutti/replications/ReplicationsFilterByBucket/ReplicationsFilterByBucket.vue';
  import ReplicationsFilterByStatus from '@/components/tutti/replications/ReplicationsFilterByStatus/ReplicationsFilterByStatus.vue';
  import ReplicationsFilterByType from '@/components/tutti/replications/ReplicationsFilterByType/ReplicationsFilterByType.vue';
  import ReplicationsFilterByDirection from '@/components/tutti/replications/ReplicationsFilterByDirection/ReplicationsFilterByDirection.vue';
  import { useTuttiReplicationsStore } from '@/stores/tuttiReplicationsStore';

  const { t } = useI18n({ messages: i18nReplications });
  const { isLoading } = storeToRefs(useTuttiReplicationsStore());
</script>

<template>
  <div
    class="replications-filters"
    :aria-busy="isLoading"
  >
    <NFormItem
      :label="t('filterLabelStatus')"
      :show-feedback="false"
    >
      <ReplicationsFilterByStatus />
    </NFormItem>
    <NFormItem
      :label="t('filterLabelType')"
      :show-feedback="false"
    >
      <ReplicationsFilterByType />
    </NFormItem>
    <NFormItem
      :label="t('filterLabelUser')"
      :show-feedback="false"
    >
      <ReplicationsFilterByUser />
    </NFormItem>
    <NFormItem
      :label="t('filterLabelBucket')"
      :show-feedback="false"
    >
      <ReplicationsFilterByBucket />
    </NFormItem>
    <NFormItem
      :label="t('filterLabelDirection')"
      :show-feedback="false"
    >
      <ReplicationsFilterByDirection />
    </NFormItem>
  </div>
</template>

<style lang="scss" scoped>
  .replications-filters {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: var(--tt-space-2) var(--tt-space-3);
    width: 100%;
  }
</style>
