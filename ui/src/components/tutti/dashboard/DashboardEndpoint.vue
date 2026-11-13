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
  import { storeToRefs } from 'pinia';
  import { onBeforeMount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import { CopyValue } from '@/components/kit';
  import { ChorusService } from '@/services/ChorusService';
  import { useAuthStore } from '@/stores/authStore';
  import { ErrorHelper } from '@/utils/helpers/ErrorHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nDashboard });
  const { isViewer } = storeToRefs(useAuthStore());

  const address = ref('');
  const isLoading = ref(false);
  const isForbidden = ref(false);
  const hasError = ref(false);

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
      address.value = (await ChorusService.getProxyCredentials()).address ?? '';
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
  <section
    class="dashboard-endpoint"
    :aria-label="t('endpointTitle')"
  >
    <div class="dashboard-endpoint__text">
      <h2 class="dashboard-endpoint__title">{{ t('endpointTitle') }}</h2>
      <span class="dashboard-endpoint__hint">{{
        isForbidden ? t('endpointAdminNote') : t('endpointText')
      }}</span>
    </div>

    <span
      v-if="isLoading"
      class="dashboard-endpoint__note"
      >…</span
    >
    <span
      v-else-if="hasError"
      class="dashboard-endpoint__note"
      >{{ t('endpointError') }}</span
    >
    <CopyValue
      v-else-if="address"
      :value="address"
      :label="t('endpointTitle')"
    />
    <span
      v-else-if="!isForbidden"
      class="dashboard-endpoint__note"
      >{{ t('endpointUnavailable') }}</span
    >

    <RouterLink
      class="dashboard-endpoint__link"
      :to="{ name: RouteName.TUTTI_PROXY_ACCOUNTS }"
    >
      {{ t('endpointManage') }}
    </RouterLink>
  </section>
</template>

<style lang="scss" scoped>
  .dashboard-endpoint {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px 24px;
    padding: var(--tt-card-padding);
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);

    &__text {
      display: flex;
      flex: 1 1 280px;
      flex-direction: column;
      gap: 4px;
    }

    &__title {
      margin: 0;
      font-size: 15px;
      font-weight: 600;
    }

    &__hint {
      font-size: 13px;
      color: var(--tt-text-2);
    }

    &__note {
      font-size: 13px;
      color: var(--tt-text-3);
    }

    &__link {
      font-size: 14px;
      font-weight: 600;
      color: var(--tt-accent);
      text-decoration: none;

      &:hover {
        color: var(--tt-accent-hover);
        text-decoration: underline;
      }
    }
  }
</style>
