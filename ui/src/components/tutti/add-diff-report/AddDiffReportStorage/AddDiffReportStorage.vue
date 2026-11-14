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
  import { computed } from 'vue';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import type { TuttiStorage } from '@/utils/types/tuttiCore';
  import TuttiStorageCard from '@/components/tutti/common/TuttiStorageCard/TuttiStorageCard.vue';

  const props = defineProps<{
    modelValue: TuttiStorage | null;
    storages: TuttiStorage[];
    title: string;
    description: string;
    hasError: boolean;
    errorMessage: string;
    disabledStorageName?: string;
  }>();

  const emit = defineEmits<{
    (e: 'update:modelValue', value: TuttiStorage | null): void;
  }>();

  const storageOptions = computed(() =>
    GeneralHelper.orderBy(props.storages, 'isMain', 'desc'),
  );

  function selectStorage(storage: TuttiStorage) {
    if (props.modelValue?.name === storage.name) {
      emit('update:modelValue', null);
    } else {
      emit('update:modelValue', storage);
    }
  }
</script>

<template>
  <div class="add-diff-report-storage">
    <h5 class="add-diff-report-storage__title">{{ title }}</h5>
    <p class="add-diff-report-storage__description">{{ description }}</p>

    <div class="add-diff-report-storage__cards">
      <TuttiStorageCard
        v-for="storage in storageOptions"
        :key="storage.name"
        :storage="storage"
        is-selectable
        :type="storage.isMain ? 'success' : 'warning'"
        :is-selected="storage.name === modelValue?.name"
        :is-disabled="storage.name === disabledStorageName"
        @select="selectStorage(storage)"
      />
    </div>

    <span
      v-if="hasError"
      class="add-diff-report-storage__error"
    >
      {{ errorMessage }}
    </span>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .add-diff-report-storage {
    &__title,
    &__description {
      margin-bottom: utils.unit(2);
    }

    &__cards {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(264px, 1fr));
      gap: utils.unit(3);
    }

    &__error {
      color: var(--error-color);
      font-size: 12px;
    }
  }
</style>
