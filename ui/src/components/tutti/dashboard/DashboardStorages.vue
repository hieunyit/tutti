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
  import { CTile } from '@clyso/clyso-ui-kit';
  import { NTag } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import type {
    StorageHealthRow,
    StorageHealthState,
  } from '@/utils/helpers/DashboardHelper';
  import { RouteName } from '@/utils/types/router';

  defineProps<{
    storages: StorageHealthRow[] | null;
  }>();

  const { t } = useI18n({ messages: i18nDashboard });

  const TAG_TYPE: Record<StorageHealthState, 'success' | 'error' | 'default'> =
    {
      healthy: 'success',
      unavailable: 'error',
      unknown: 'default',
    };

  const STATE_KEY: Record<StorageHealthState, string> = {
    healthy: 'stateHealthy',
    unavailable: 'stateUnavailable',
    unknown: 'stateUnknown',
  };
</script>

<template>
  <CTile class="dashboard-storages">
    <template #title>{{ t('storagesTitle') }}</template>
    <template #header-extra>
      <RouterLink
        class="dashboard-storages__manage"
        :to="{ name: RouteName.CHORUS_STORAGES }"
      >
        {{ t('storagesManage') }} →
      </RouterLink>
    </template>

    <p
      v-if="storages === null"
      class="dashboard-storages__empty"
    >
      {{ t('storagesUnavailable') }}
    </p>

    <p
      v-else-if="storages.length === 0"
      class="dashboard-storages__empty"
    >
      {{ t('storagesEmpty') }}
    </p>

    <ul
      v-else
      class="dashboard-storages__list"
    >
      <li
        v-for="storage in storages"
        :key="storage.name"
        class="dashboard-storages__row"
      >
        <div class="dashboard-storages__info">
          <div class="dashboard-storages__name-line">
            <RouterLink
              class="dashboard-storages__name"
              :to="{
                name: RouteName.CHORUS_STORAGE_DETAILS,
                params: { storageName: storage.name },
              }"
            >
              {{ storage.name }}
            </RouterLink>
            <NTag
              v-if="storage.isMain"
              size="small"
              type="info"
              :bordered="false"
            >
              {{ t('storageMain') }}
            </NTag>
          </div>
          <span class="dashboard-storages__meta">
            {{ storage.provider }} · {{ storage.address }}
          </span>
        </div>
        <NTag
          :type="TAG_TYPE[storage.state]"
          size="small"
          :bordered="false"
          round
        >
          {{ t(STATE_KEY[storage.state]) }}
        </NTag>
      </li>
    </ul>
  </CTile>
</template>

<style lang="scss" scoped>
  .dashboard-storages {
    &__manage {
      font-size: 13px;
      color: var(--primary-color);
      text-decoration: none;

      &:hover,
      &:focus-visible {
        text-decoration: underline;
      }
    }

    &__empty {
      margin: 0;
      color: var(--text-color-3);
    }

    &__list {
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 0;

      & + & {
        border-top: 1px solid var(--border-color);
      }
    }

    &__info {
      display: flex;
      flex-direction: column;
      gap: 2px;
      min-width: 0;
    }

    &__name-line {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-wrap: wrap;
    }

    &__name {
      overflow-wrap: anywhere;
      font-weight: 600;
      color: var(--text-color-1);
      text-decoration: none;

      &:hover,
      &:focus-visible {
        color: var(--primary-color);
      }
    }

    &__meta {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 12px;
      color: var(--text-color-3);
    }
  }
</style>
