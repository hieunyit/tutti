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
  import { CSkeleton } from '@clyso/clyso-ui-kit';
  import { useChorusReplicationsStore } from '@/stores/chorusReplicationsStore';
  import ReplicationsFilterByBucket from '@/components/chorus/replications/ReplicationsFilterByBucket/ReplicationsFilterByBucket.vue';
  import ReplicationsFilterByStatus from '@/components/chorus/replications/ReplicationsFilterByStatus/ReplicationsFilterByStatus.vue';
  import ReplicationsFilterByCreatedAt from '@/components/chorus/replications/ReplicationsFilterByCreatedAt/ReplicationsFilterByCreatedAt.vue';
  import ReplicationsFilterByType from '@/components/chorus/replications/ReplicationsFilterByType/ReplicationsFilterByType.vue';
  import ReplicationsFilterByDirection from '@/components/chorus/replications/ReplicationsFilterByDirection/ReplicationsFilterByDirection.vue';

  const { isLoading } = storeToRefs(useChorusReplicationsStore());
</script>

<template>
  <div class="replications-filters">
    <div
      v-if="isLoading"
      key="loading"
      class="replications-filters__list"
    >
      <CSkeleton
        v-for="(_, index) in Array(5)"
        :key="index"
        :height="34"
        :border-radius="4"
      />
    </div>
    <div
      v-else
      key="filters"
      class="replications-filters__list"
    >
      <ReplicationsFilterByType class="replications-filters__type" />

      <ReplicationsFilterByBucket class="replications-filters__bucket" />

      <ReplicationsFilterByCreatedAt class="replications-filters__created-at" />

      <ReplicationsFilterByDirection class="replications-filters__direction" />

      <ReplicationsFilterByStatus class="replications-filters__status" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .replications-filters {
    &__list {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
      gap: utils.unit(2) utils.unit(3);
      align-items: start;
    }
  }
</style>
