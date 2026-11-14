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
  import { computed } from 'vue';
  import { NRadio, NTag } from 'naive-ui';
  import i18nAddRoutingPolicy from '../i18nAddRoutingPolicy';
  import { useTuttiAddRoutingPolicyStore } from '@/stores/tuttiAddRoutingPolicyStore';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import type { TuttiStorage } from '@/utils/types/tuttiCore';

  const store = useTuttiAddRoutingPolicyStore();
  const { selectedToStorage, storages, isBlockOnly, validator } =
    storeToRefs(store);

  const { t } = useI18n({
    messages: i18nAddRoutingPolicy,
  });

  const storageOptions = computed(() =>
    GeneralHelper.orderBy(storages.value, 'name', 'asc'),
  );

  const storageSelectionErrorMessage = computed(() => {
    const errors = validator.value.selectedToStorage.$errors;

    if (!errors || errors.length === 0) return '';

    return errors[0]?.$message || t('unknownValidationError');
  });

  function selectStorage(storage: TuttiStorage) {
    if (selectedToStorage.value?.name === storage.name) {
      selectedToStorage.value = null;
    } else {
      selectedToStorage.value = storage;
    }
  }
</script>

<template>
  <div class="storage-selection">
    <div class="storage-selection__header">
      <h5 class="storage-selection__title">{{ t('storageSelectionTitle') }}</h5>
      <p class="storage-selection__description">
        {{ t('storageSelectionDescription') }}
      </p>
    </div>

    <div class="storage-selection__content">
      <div class="storage-selection__form-list">
        <div
          v-for="storage in storageOptions"
          :key="storage.name"
          class="storage-selection__list-row"
          :class="{
            'storage-selection__list-row--selected':
              storage.name === selectedToStorage?.name,
          }"
          role="button"
          tabindex="0"
          @click="selectStorage(storage)"
          @keydown.enter="selectStorage(storage)"
          @keydown.space.prevent="selectStorage(storage)"
        >
          <div class="storage-selection__radio-col">
            <NRadio
              :checked="storage.name === selectedToStorage?.name"
              :name="storage.name"
              @click.stop="selectStorage(storage)"
            />
          </div>

          <div class="storage-selection__meta-col">
            <div class="storage-selection__name-row">
              <span class="storage-selection__storage-name">{{
                storage.name
              }}</span>
              <NTag
                v-if="storage.provider"
                size="small"
                round
              >
                {{ storage.provider }}
              </NTag>
            </div>
            <div
              v-if="storage.address"
              class="storage-selection__address-row"
            >
              <code>{{ storage.address }}</code>
            </div>
          </div>
        </div>
      </div>
    </div>

    <span
      v-if="validator.selectedToStorage.$error"
      class="storage-selection__error-text"
    >
      {{ storageSelectionErrorMessage }}
    </span>
    <span
      v-if="isBlockOnly"
      class="storage-selection__hint-text"
    >
      {{ t('storageSelectionBlockedOnlyHint') }}
    </span>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .storage-selection {
    &__title,
    &__description {
      margin-bottom: utils.unit(1);
    }

    &__content {
      display: flex;
      flex-direction: column;
      max-width: 680px;
    }

    &__form-list {
      display: flex;
      flex-direction: column;
      gap: utils.unit(1.5);
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

      &:hover {
        background-color: var(--tt-hover-bg, rgba(0, 0, 0, 0.03));
      }

      &--selected {
        background-color: var(--tt-selected-bg, rgba(16, 185, 129, 0.08));
        border-color: var(--tt-primary, #10b981);
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

    &__error-text {
      color: var(--error-color, #ef4444);
      font-size: 12px;
      margin-top: utils.unit(1);
      display: block;
    }

    &__hint-text {
      color: var(--warning-color, #f59e0b);
      font-size: 12px;
      margin-top: utils.unit(1);
      display: block;
    }
  }
</style>
