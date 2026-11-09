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
  import type {
    DataTableBaseColumn,
    DataTableSortState,
  } from '@clyso/clyso-ui-kit';
  import { CButton, CDataTable, CIcon, CResult } from '@clyso/clyso-ui-kit';
  import { computed } from 'vue';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import i18nCredentials from '../i18nCredentials';
  import CredentialsActionsCell from '../CredentialsActionsCell/CredentialsActionsCell.vue';
  import type { ChorusCredential } from '@/utils/types/chorus';
  import { useChorusStorageDetailsStore } from '@/stores/chorusStorageDetailsStore';
  import { IconName } from '@/utils/types/icon';
  import { RouteName } from '@/utils/types/router';
  import WriteGate from '@/components/auth/WriteGate.vue';

  const { t } = useI18n({
    messages: i18nCredentials,
  });

  const {
    isLoading,
    hasError,
    storage,
    credentialsSorter,
    credentialsPage,
    credentialsPageSize,
    credentialsPagination,
    computedCredentials,
    credentialsFilterAliases,
  } = storeToRefs(useChorusStorageDetailsStore());
  const { initStorageDetails } = useChorusStorageDetailsStore();

  const columns = computed<DataTableBaseColumn<ChorusCredential>[]>(() => [
    {
      title: t('columnUserAlias'),
      key: 'alias',
      sorter: true,
    },
    {
      title: t('columnAccessKey'),
      key: 'accessKey',
      sorter: true,
    },
    {
      title: t('columnActions'),
      key: 'actions',
    },
  ]);

  const handleSortingChange = (newSorter: DataTableSortState | null) => {
    if (!computedCredentials.value.length) {
      return;
    }

    credentialsSorter.value = newSorter;
    credentialsPage.value = 1;
  };

  const handlePageChange = (newPage: number) => {
    credentialsPage.value = newPage;
  };

  const handlePageSizeUpdate = (newPageSize: number) => {
    credentialsPageSize.value = newPageSize;
    credentialsPage.value = 1;
  };

  const rowKey = (row: ChorusCredential) => row.alias;
</script>

<template>
  <div class="credentials-list">
    <div class="credentials-list__container">
      <CDataTable
        class="credentials-list__table"
        :columns="columns"
        :data="computedCredentials"
        max-height="1020px"
        virtual-scroll
        :is-controlled="true"
        :is-loading="isLoading"
        :has-error="hasError"
        :bordered="false"
        :sorter="credentialsSorter"
        :row-key="rowKey"
        :pagination="credentialsPagination"
        @update:sorter="handleSortingChange"
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeUpdate"
        @retry="initStorageDetails(storage?.name ?? '')"
      >
        <template #alias="{ rowData }">
          {{ rowData.alias }}
        </template>

        <template #accessKey="{ rowData }">
          {{ rowData.accessKey }}
        </template>

        <template #actions="{ rowData }">
          <CredentialsActionsCell :credential="rowData as ChorusCredential" />
        </template>

        <template #empty>
          <CResult
            class="credentials-list__result"
            type="empty"
            size="medium"
            :icon-name="IconName.BASE_KEY"
            :max-width="480"
          >
            <template #title>
              {{
                t(
                  credentialsFilterAliases.length
                    ? 'filterNoResultsTitle'
                    : 'noCredentialsTitle',
                )
              }}
            </template>

            <p>
              {{
                t(
                  credentialsFilterAliases.length
                    ? 'filterNoResultsText'
                    : 'noCredentialsText',
                )
              }}
            </p>

            <template
              v-if="storage && !credentialsFilterAliases.length"
              #actions
            >
              <WriteGate>
                <RouterLink
                  :to="{
                    name: RouteName.CHORUS_SET_CREDENTIAL,
                    params: { storageName: storage.name },
                  }"
                >
                  <CButton
                    type="primary"
                    tag="div"
                  >
                    <template #icon>
                      <CIcon
                        :is-inline="true"
                        :name="IconName.BASE_ADD"
                      />
                    </template>

                    {{ t('addCredentialAction') }}
                  </CButton>
                </RouterLink>
              </WriteGate>
            </template>
          </CResult>
        </template>
      </CDataTable>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .credentials-list {
    &__container {
      overflow-x: auto;
    }

    &__table {
      min-width: 600px;
    }

    &__result {
      min-height: 280px;
    }
  }
</style>
