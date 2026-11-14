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
  import { useI18n } from 'vue-i18n';
  import { computed } from 'vue';
  import { NSelect, NTag } from 'naive-ui';
  import i18nAddRoutingPolicy from '../i18nAddRoutingPolicy';
  import { useTuttiAddRoutingPolicyStore } from '@/stores/tuttiAddRoutingPolicyStore';

  const { selectedUser, users, validator } = storeToRefs(
    useTuttiAddRoutingPolicyStore(),
  );

  const { t } = useI18n({
    messages: i18nAddRoutingPolicy,
  });

  const userOptions = computed(() =>
    users.value.map((user) => ({
      label: user,
      value: user,
    })),
  );

  const userSelectionErrorMessage = computed(() => {
    const errors = validator.value.selectedUser.$errors;

    if (!errors || errors.length === 0) return '';

    return errors[0]?.$message || t('unknownValidationError');
  });

  function selectQuickUser(user: string) {
    selectedUser.value = user;
  }
</script>

<template>
  <div class="user-selection">
    <div class="user-selection__header">
      <h5 class="user-selection__title">{{ t('userSelectionTitle') }}</h5>
      <p class="user-selection__description">
        {{ t('userSelectionDescription') }}
      </p>
    </div>

    <div class="user-selection__content">
      <div class="user-selection__form-control">
        <NSelect
          v-model:value="selectedUser"
          filterable
          tag
          clearable
          :options="userOptions"
          :placeholder="
            t('userSelectionPlaceholder') || 'Select or enter an S3 user'
          "
        />
      </div>

      <div
        v-if="users.length > 0"
        class="user-selection__quick-list"
      >
        <span class="user-selection__quick-label">Available users:</span>
        <div class="user-selection__tags">
          <NTag
            v-for="u in users"
            :key="u"
            :type="u === selectedUser ? 'primary' : 'default'"
            round
            checkable
            :checked="u === selectedUser"
            style="cursor: pointer"
            @click="selectQuickUser(u)"
          >
            {{ u }}
          </NTag>
        </div>
      </div>

      <span
        v-if="validator.selectedUser.$error"
        class="user-selection__error-text"
      >
        {{ userSelectionErrorMessage }}
      </span>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .user-selection {
    &__title,
    &__description {
      margin-bottom: utils.unit(1);
    }

    &__content {
      display: flex;
      flex-direction: column;
      gap: utils.unit(2);
      max-width: 600px;
    }

    &__form-control {
      width: 100%;
    }

    &__quick-list {
      display: flex;
      flex-direction: column;
      gap: utils.unit(1);
    }

    &__quick-label {
      font-size: 12px;
      color: var(--tt-text-3, #71717a);
      font-weight: 500;
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: utils.unit(1.5);
    }

    &__error-text {
      color: var(--error-color, #ef4444);
      font-size: 12px;
    }
  }
</style>
