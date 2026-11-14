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
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NRadio, NTag } from 'naive-ui';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import i18nAddReplication from '@/components/tutti/add-replication/i18nAddReplication';
  import type { TuttiStorage } from '@/utils/types/tuttiCore';

  const { storages, selectedToStorage, selectedFromStorage } = storeToRefs(
    useTuttiAddReplicationStore(),
  );
  const { t } = useI18n({
    messages: i18nAddReplication,
  });

  const toStoragesOptions = computed(() =>
    GeneralHelper.orderBy(storages.value, 'name', 'asc'),
  );

  function isSameAsSource(storage: TuttiStorage): boolean {
    return storage.name === selectedFromStorage.value?.name;
  }

  function selectStorage(storage: TuttiStorage) {
    if (isSameAsSource(storage)) return;

    selectedToStorage.value = storage;
  }
</script>

<template>
  <div class="to-storage-form-step">
    <p class="to-storage-form-step__title">
      {{ t('toStorageStepTitle') }}
    </p>

    <div class="to-storage-form-step__form-list">
      <div
        v-for="storage in toStoragesOptions"
        :key="storage.name"
        class="to-storage-form-step__list-row"
        :class="{
          'to-storage-form-step__list-row--selected':
            storage.name === selectedToStorage?.name,
          'to-storage-form-step__list-row--disabled': isSameAsSource(storage),
        }"
        role="button"
        :tabindex="isSameAsSource(storage) ? -1 : 0"
        @click="selectStorage(storage)"
        @keydown.enter="selectStorage(storage)"
        @keydown.space.prevent="selectStorage(storage)"
      >
        <div class="to-storage-form-step__radio-col">
          <NRadio
            :checked="storage.name === selectedToStorage?.name"
            :name="storage.name"
            :disabled="isSameAsSource(storage)"
            @click.stop="selectStorage(storage)"
          />
        </div>

        <div class="to-storage-form-step__meta-col">
          <div class="to-storage-form-step__name-row">
            <span class="to-storage-form-step__storage-name">{{
              storage.name
            }}</span>
            <NTag
              v-if="storage.provider"
              size="small"
              round
            >
              {{ storage.provider }}
            </NTag>
            <NTag
              v-if="isSameAsSource(storage)"
              size="small"
              type="warning"
              round
            >
              Source Storage
            </NTag>
          </div>
          <div
            v-if="storage.address"
            class="to-storage-form-step__address-row"
          >
            <code>{{ storage.address }}</code>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .to-storage-form-step {
    &__title {
      margin-bottom: utils.unit(2);
      font-weight: 500;
    }

    &__form-list {
      display: flex;
      flex-direction: column;
      gap: utils.unit(1.5);
      max-width: 680px;
      border: 1px solid var(--tt-border, #e4e4e7);
      border-radius: 8px;
      padding: utils.unit(1.5);
      background-color: var(--tt-surface, #ffffff);
    }

    &__list-row {
      display: flex;
      align-items: center;
      gap: utils.unit(2);
      padding: utils.unit(2);
      border-radius: 6px;
      border: 1px solid transparent;
      cursor: pointer;
      transition:
        background-color 0.15s,
        border-color 0.15s;

      &:hover:not(&--disabled) {
        background-color: var(--tt-hover-bg, rgba(0, 0, 0, 0.03));
      }

      &--selected {
        background-color: var(--tt-selected-bg, rgba(16, 185, 129, 0.08));
        border-color: var(--tt-primary, #10b981);
      }

      &--disabled {
        opacity: 0.55;
        cursor: not-allowed;
        background-color: var(--tt-subtle, rgba(0, 0, 0, 0.02));
      }
    }

    &__radio-col {
      display: flex;
      align-items: center;
      flex: none;
    }

    &__meta-col {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 0;
      flex: 1;
    }

    &__name-row {
      display: flex;
      align-items: center;
      gap: utils.unit(1.5);
      flex-wrap: wrap;
    }

    &__storage-name {
      font-weight: 600;
      font-size: 14px;
      color: var(--tt-text, #18181b);
    }

    &__address-row {
      font-size: 12px;
      color: var(--tt-text-3, #71717a);

      code {
        font-family: monospace;
      }
    }
  }
</style>
