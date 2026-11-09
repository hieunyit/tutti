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
  import { CButton, CSkeleton, CTile } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { onBeforeMount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import CopyButton from '@/components/chorus/common/CopyButton/CopyButton.vue';
  import ProxyCredentialsModal from '@/components/chorus/home/ProxyWidget/ProxyCredentialsModal.vue';
  import { ChorusService } from '@/services/ChorusService';
  import { useAuthStore } from '@/stores/authStore';
  import { ErrorHelper } from '@/utils/helpers/ErrorHelper';
  import type { ChorusProxyCredentials } from '@/utils/types/chorus';

  const { t } = useI18n({ messages: i18nDashboard });
  const { isViewer } = storeToRefs(useAuthStore());

  const proxy = ref<ChorusProxyCredentials | null>(null);
  const isLoading = ref(false);
  const isForbidden = ref(false);
  const hasError = ref(false);
  const isModalShown = ref(false);

  async function load() {
    hasError.value = false;
    isForbidden.value = false;

    // the endpoint is admin-only: do not request it for viewers
    if (isViewer.value) {
      isForbidden.value = true;

      return;
    }

    isLoading.value = true;

    try {
      proxy.value = await ChorusService.getProxyCredentials();
    } catch (error: unknown) {
      if (ErrorHelper.getStatusCode(error) === 403) isForbidden.value = true;
      else hasError.value = true;
    } finally {
      isLoading.value = false;
    }
  }

  onBeforeMount(load);
</script>

<template>
  <CTile class="dashboard-endpoint">
    <template #title>{{ t('endpointTitle') }}</template>

    <CSkeleton
      v-if="isLoading"
      type="text"
      :repeat="2"
    />

    <p
      v-else-if="isForbidden"
      class="dashboard-endpoint__note"
    >
      {{ t('endpointAdminOnly') }}
    </p>

    <p
      v-else-if="hasError"
      class="dashboard-endpoint__note"
    >
      {{ t('endpointError') }}
    </p>

    <template v-else-if="proxy">
      <div class="dashboard-endpoint__row">
        <span class="dashboard-endpoint__label">
          {{ t('endpointAddress') }}
        </span>
        <span class="dashboard-endpoint__value">
          <code class="dashboard-endpoint__address">
            {{ proxy.address || t('endpointUnavailable') }}
          </code>
          <CopyButton
            v-if="proxy.address"
            :text="proxy.address"
          />
        </span>
      </div>
      <div class="dashboard-endpoint__row">
        <span class="dashboard-endpoint__label">
          {{ t('endpointCredentials') }}
        </span>
        <CButton
          text
          size="small"
          type="primary"
          @click="isModalShown = true"
        >
          {{ t('endpointSeeCredentials') }}
        </CButton>
      </div>

      <ProxyCredentialsModal
        v-model:is-shown="isModalShown"
        :proxy="proxy"
      />
    </template>
  </CTile>
</template>

<style lang="scss" scoped>
  .dashboard-endpoint {
    &__note {
      margin: 0;
      color: var(--text-color-3);
    }

    &__row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 6px 0;
    }

    &__label {
      flex: none;
      font-size: 13px;
      color: var(--text-color-3);
    }

    &__value {
      display: flex;
      align-items: center;
      gap: 4px;
      min-width: 0;
    }

    &__address {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 13px;
      color: var(--text-color-1);
    }
  }
</style>
