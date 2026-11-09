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
  import { CButton } from '@clyso/clyso-ui-kit';
  import { NEmpty } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { onBeforeMount, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nHealth from '@/components/tutti/health/i18nHealth';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import ProxyHealthCard from '@/components/tutti/health/ProxyHealthCard.vue';
  import ChorusListError from '@/components/chorus/common/ChorusListError/ChorusListError.vue';
  import { useTuttiHealthStore } from '@/stores/tuttiHealthStore';

  const { t } = useI18n({ messages: i18nHealth });
  const store = useTuttiHealthStore();
  const { proxies, hasError, hasNoData, isLoading } = storeToRefs(store);

  onBeforeMount(() => store.startPolling());
  onBeforeUnmount(() => store.$reset());
</script>

<template>
  <div class="tutti-health-page">
    <PageHeader :title="t('title')">
      <template #actions>
        <CButton
          :loading="isLoading"
          @click="store.loadProxies(true)"
        >
          {{ t('refresh') }}
        </CButton>
      </template>
    </PageHeader>

    <ChorusListError
      v-if="hasError && hasNoData"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-text="t('retry')"
      @retry="store.loadProxies(true)"
    />
    <NEmpty
      v-else-if="!isLoading && hasNoData"
      :description="t('empty')"
    />
    <ProxyHealthCard
      v-for="proxy in proxies"
      :key="proxy.appId"
      :proxy="proxy"
      class="tutti-health-page__card"
    />
  </div>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .tutti-health-page {
    &__card {
      margin-bottom: utils.unit(4);
    }
  }
</style>
