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
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import { RouteName } from '@/utils/types/router';
  import { IconName } from '@/utils/types/icon';
  import i18nSetCredential from '@/components/chorus/set-credential/i18nSetCredential';
  import { useChorusSetCredentialStore } from '@/stores/chorusSetCredentialStore';
  import CredentialFormTile from '@/components/chorus/set-credential/CredentialFormTile/CredentialFormTile.vue';

  const props = defineProps<{
    storageName: string;
    alias?: string;
  }>();

  const { t } = useI18n({ messages: i18nSetCredential });
  const { initSetCredentialPage, $reset } = useChorusSetCredentialStore();
  const { isEditMode } = storeToRefs(useChorusSetCredentialStore());

  onBeforeMount(() => {
    initSetCredentialPage(props.storageName, props.alias);
  });

  onUnmounted(() => {
    $reset();
  });
</script>

<template>
  <div class="chorus-set-credential-page">
    <PageHeader
      :title="
        isEditMode
          ? t('setCredentialTitleEdit', { user: props.alias })
          : t('setCredentialTitleAdd')
      "
      :description="t('setCredentialHeader', { storage: props.storageName })"
    >
      <template #actions>
        <RouterLink
          :to="{
            name: RouteName.CHORUS_STORAGE_DETAILS,
            params: { storageName: props.storageName },
          }"
        >
          <CButton tag="div">
            <template #icon>
              <CIcon
                :is-inline="true"
                :name="IconName.BASE_ARROW_BACK"
              />
            </template>

            {{ props.storageName }}
          </CButton>
        </RouterLink>
      </template>
    </PageHeader>

    <CredentialFormTile class="chorus-set-credential-page__form" />
  </div>
</template>

<style lang="scss" scoped>
  .chorus-set-credential-page {
    &__form {
      max-width: 600px;
    }
  }
</style>
