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
  import { CAlert, CResult } from '@clyso/clyso-ui-kit';
  import ChorusUserCard from '../../common/ChorusUserCard/ChorusUserCard.vue';
  import { useChorusAddReplicationStore } from '@/stores/chorusAddReplicationStore';
  import i18nAddReplication from '@/components/chorus/add-replication/i18nAddReplication';

  const store = useChorusAddReplicationStore();
  const {
    users,
    hasUsersError,
    selectedUser,
    selectedFromStorage,
    selectedToStorage,
    isSelectedUserUsable,
  } = storeToRefs(store);

  const { t } = useI18n({
    messages: i18nAddReplication,
  });

  const modeLabels: Record<string, string> = {
    override: 'userModeOverride',
    'connection-account': 'userModeConnection',
    missing: 'userModeMissing',
  };

  const storageNames = computed(() =>
    [selectedFromStorage.value?.name, selectedToStorage.value?.name].filter(
      (name): name is string => !!name,
    ),
  );

  const userRows = computed(() =>
    users.value.map((user) => ({
      name: user.name,
      modes: storageNames.value.map((storage) => ({
        storage,
        label: t(
          modeLabels[store.userModeOn(user.name, storage)] ?? 'userModeUnknown',
        ),
      })),
    })),
  );

  const showMissingWarning = computed(
    () => selectedUser.value !== null && !isSelectedUserUsable.value,
  );
</script>

<template>
  <div class="user-form-step">
    <p class="user-form-step__title">
      {{ t('userStepTitle') }}
    </p>

    <CResult
      v-if="hasUsersError || !userRows.length"
      status="error"
      type="error"
      :max-width="600"
      :has-positive="false"
    >
      <template #title>
        {{ t('noUsersTitle') }}
      </template>

      <p>{{ hasUsersError ? t('usersLoadError') : t('noUsersText') }}</p>
    </CResult>

    <div
      v-else
      class="user-form-step__list"
    >
      <ChorusUserCard
        v-for="row in userRows"
        :key="row.name"
        :user="row.name"
        :is-selectable="true"
        :is-selected="row.name === selectedUser"
        @select="selectedUser = row.name"
      >
        <div class="user-form-step__option">
          <span class="user-form-step__name">{{ row.name }}</span>
          <small
            v-for="mode in row.modes"
            :key="mode.storage"
            class="user-form-step__mode"
          >
            {{ mode.storage }}: {{ mode.label }}
          </small>
        </div>
      </ChorusUserCard>
    </div>

    <CAlert
      v-if="showMissingWarning"
      type="warning"
      class="user-form-step__warning"
    >
      {{ t('userMissingWarning') }}
    </CAlert>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .user-form-step {
    &__title {
      margin-bottom: utils.unit(2);
    }

    &__list {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
      gap: utils.unit(3);
    }

    &__option {
      display: flex;
      flex-direction: column;
      gap: utils.unit(1);
    }

    &__name {
      font-weight: 600;
    }

    &__warning {
      margin-top: utils.unit(2);
    }
  }
</style>
