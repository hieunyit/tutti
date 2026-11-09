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
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import { NTooltip } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from '@/components/tutti/accounts/i18nAccounts';
  import ProxyAccountsTile from '@/components/tutti/accounts/ProxyAccountsTile.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import Callout from '@/components/ui/Callout.vue';
  import ErrorState from '@/components/tutti/common/ErrorState.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { IconName } from '@/utils/types/icon';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { hasError, isLoading, dynamicAvailable, dynamicReason } =
    storeToRefs(store);

  const accountsTile = ref<InstanceType<typeof ProxyAccountsTile> | null>(null);

  onBeforeMount(() => store.init());
  onBeforeUnmount(() => store.$reset());
</script>

<template>
  <div class="tutti-proxy-accounts-page">
    <PageHeader
      :title="t('title')"
      :description="t('pageDescription')"
    >
      <template #actions>
        <CButton
          :loading="isLoading"
          @click="store.init()"
        >
          {{ t('refresh') }}
        </CButton>
        <WriteGate v-if="!hasError">
          <NTooltip :disabled="dynamicAvailable">
            <template #trigger>
              <span class="tutti-proxy-accounts-page__create">
                <CButton
                  type="primary"
                  :disabled="!dynamicAvailable"
                  @click="accountsTile?.openCreate()"
                >
                  <template #icon>
                    <CIcon
                      :is-inline="true"
                      :name="IconName.BASE_ADD"
                    />
                  </template>

                  {{ t('createAccount') }}
                </CButton>
              </span>
            </template>
            {{
              t('unavailableReason', {
                reason: dynamicReason || t('notConfigured'),
              })
            }}
          </NTooltip>
        </WriteGate>
      </template>
    </PageHeader>

    <ErrorState
      v-if="hasError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-label="t('retry')"
      @retry="store.init()"
    />
    <div
      v-else
      class="tutti-proxy-accounts-page__content"
    >
      <Callout
        v-if="!dynamicAvailable"
        tone="warning"
        :title="t('unavailableTitle')"
      >
        {{ t('unavailableText') }}
      </Callout>
      <ProxyAccountsTile ref="accountsTile" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .tutti-proxy-accounts-page {
    &__content {
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    &__create {
      display: inline-flex;
    }
  }
</style>
