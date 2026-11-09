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
  import { onBeforeMount, onUnmounted } from 'vue';
  import { storeToRefs } from 'pinia';
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import { useI18n } from 'vue-i18n';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import StorageDetailsError from '@/components/chorus/storage-details/StorageDetailsError/StorageDetailsError.vue';
  import { useChorusStorageDetailsStore } from '@/stores/chorusStorageDetailsStore';
  import { RouteName } from '@/utils/types/router';
  import { IconName } from '@/utils/types/icon';
  import i18nStorageDetails from '@/components/chorus/storage-details/i18nStorageDetails';
  import StorageGeneral from '@/components/chorus/storage-details/StorageGeneral/StorageGeneral.vue';
  import CredentialsTile from '@/components/chorus/credentials/CredentialsTile/CredentialsTile.vue';

  const props = defineProps<{
    storageName: string;
  }>();

  const { t } = useI18n({
    messages: i18nStorageDetails,
  });

  const { hasError, storage } = storeToRefs(useChorusStorageDetailsStore());
  const { initStorageDetails, $reset } = useChorusStorageDetailsStore();

  onBeforeMount(() => {
    initStorageDetails(props.storageName);
  });

  onUnmounted(() => {
    $reset();
  });
</script>

<template>
  <div class="storage-details-page">
    <PageHeader
      :title="storageName"
      :description="t('pageDescription')"
    >
      <template #actions>
        <RouterLink :to="{ name: RouteName.CHORUS_STORAGES }">
          <CButton tag="div">
            <template #icon>
              <CIcon
                :is-inline="true"
                :name="IconName.BASE_ARROW_BACK"
              />
            </template>

            {{ t('backToStorages') }}
          </CButton>
        </RouterLink>
        <WriteGate v-if="storage && !hasError">
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
    </PageHeader>

    <StorageDetailsError
      v-if="hasError"
      key="error"
    />
    <div
      v-else
      key="content"
      class="storage-details-page__content"
    >
      <StorageGeneral class="storage-details-page__general" />
      <CredentialsTile class="storage-details-page__credentials" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .storage-details-page {
    &__general {
      margin-bottom: utils.unit(3);
    }
  }
</style>
