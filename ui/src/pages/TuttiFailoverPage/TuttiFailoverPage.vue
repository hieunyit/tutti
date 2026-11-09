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
  import { NAlert } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from '@/components/tutti/failover/i18nFailover';
  import FailoverPoliciesTile from '@/components/tutti/failover/FailoverPoliciesTile.vue';
  import FailoverEventsTile from '@/components/tutti/failover/FailoverEventsTile.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import ChorusListError from '@/components/chorus/common/ChorusListError/ChorusListError.vue';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { IconName } from '@/utils/types/icon';

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { hasError, isLoading, proxiesWithFailoverDisabled } =
    storeToRefs(store);
  const policiesTile = ref<InstanceType<typeof FailoverPoliciesTile> | null>(
    null,
  );

  onBeforeMount(() => store.init());
  onBeforeUnmount(() => store.$reset());
</script>

<template>
  <div class="tutti-failover-page">
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
          <CButton
            type="primary"
            @click="policiesTile?.openAdd()"
          >
            <template #icon>
              <CIcon
                :is-inline="true"
                :name="IconName.BASE_ADD"
              />
            </template>

            {{ t('addPolicy') }}
          </CButton>
        </WriteGate>
      </template>
    </PageHeader>

    <ChorusListError
      v-if="hasError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-text="t('retry')"
      @retry="store.init()"
    />
    <template v-else>
      <NAlert
        type="info"
        class="tutti-failover-page__warning"
      >
        {{ t('help') }}
      </NAlert>
      <NAlert
        v-if="proxiesWithFailoverDisabled > 0"
        type="warning"
        class="tutti-failover-page__warning"
      >
        {{ t('disabledWarning', { count: proxiesWithFailoverDisabled }) }}
      </NAlert>

      <FailoverPoliciesTile
        ref="policiesTile"
        class="tutti-failover-page__tile"
      />
      <FailoverEventsTile class="tutti-failover-page__tile" />
    </template>
  </div>
</template>

<style lang="scss" scoped>
  .tutti-failover-page {
    &__warning,
    &__tile {
      margin-bottom: 16px;
    }
  }
</style>
