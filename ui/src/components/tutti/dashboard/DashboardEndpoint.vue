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
  import { TuttiCoreService } from '@/services/TuttiCoreService';
  import { useAuthStore } from '@/stores/authStore';

  const { t } = useI18n({ messages: i18nDashboard });
  const { isViewer } = storeToRefs(useAuthStore());

  const address = ref('');

  /**
   * The address is admin-only and a convenience: viewers, errors and a
   * missing address all simply show nothing.
   */
  async function load() {
    if (isViewer.value) return;

    try {
      address.value =
        (await TuttiCoreService.getProxyCredentials()).address ?? '';
    } catch {
      address.value = '';
    }
  }

  onBeforeMount(load);
</script>

<template>
  <div
    v-if="address"
    class="dashboard-endpoint"
  >
    <span class="dashboard-endpoint__label">{{ t('endpointTitle') }}</span>
    <CopyValue
      :value="address"
      :label="t('endpointTitle')"
    />
  </div>
</template>

<style lang="scss" scoped>
  .dashboard-endpoint {
    display: flex;
    align-items: center;
    gap: var(--tt-space-2);

    &__label {
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
      white-space: nowrap;
    }
  }
</style>
