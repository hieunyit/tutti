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
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import i18nAccounts from './i18nAccounts';
  import ProxyAccountCreateModal from './ProxyAccountCreateModal.vue';
  import ProxyAccountKeysTable from './ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from './ProxyKeyRotateModal.vue';
  import ProxySecretModal from './ProxySecretModal.vue';
  import { EmptyBlock, FilterBar } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { type TuttiProxyAccountSecret } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { accounts, proxyAddress, dynamicAvailable } = storeToRefs(store);
  const query = ref('');

  const showCreate = ref(false);
  const rotateKeyId = ref<string | null>(null);
  /** one-time secret being displayed; cleared on close, never persisted */
  const shownSecret = ref<TuttiProxyAccountSecret | null>(null);
  const shownSecretKind = ref<'created' | 'rotated'>('created');

  const userNames = computed(() => accounts.value.map((a) => a.user));
  const visibleAccounts = computed(() =>
    ProxyAccountHelper.filterAccounts(accounts.value, query.value),
  );
  const hasKeys = computed(() =>
    accounts.value.some((account) => (account.accessKeyIds ?? []).length > 0),
  );

  // the primary "Create account" action lives in the page header
  defineExpose({ openCreate: () => (showCreate.value = true) });

  function onCreated(secret: TuttiProxyAccountSecret) {
    shownSecretKind.value = 'created';
    shownSecret.value = secret;
  }

  function onRotated(secret: TuttiProxyAccountSecret) {
    rotateKeyId.value = null;
    shownSecretKind.value = 'rotated';
    shownSecret.value = secret;
  }
</script>

<template>
  <div class="accounts-tile">
    <EmptyBlock
      v-if="!hasKeys"
      :title="t('emptyTitle')"
      :description="t('emptyText')"
    >
      <WriteGate>
        <NButton
          type="primary"
          :disabled="!dynamicAvailable"
          @click="showCreate = true"
          >{{ t('createAccount') }}</NButton
        >
      </WriteGate>
    </EmptyBlock>
    <template v-else>
      <FilterBar
        v-model:search="query"
        :search-label="t('filterLabel')"
        :search-placeholder="t('filterPlaceholder')"
        @clear="query = ''"
      />
      <p class="accounts-tile__note">{{ t('filterNote') }}</p>
      <EmptyBlock
        v-if="visibleAccounts.length === 0"
        :title="t('noResultsTitle')"
        :description="t('noResultsText')"
      >
        <NButton @click="query = ''">{{ t('noResultsAction') }}</NButton>
      </EmptyBlock>
      <ProxyAccountKeysTable
        v-else
        :accounts="visibleAccounts"
        @rotate="rotateKeyId = $event"
      />
    </template>

    <ProxyAccountCreateModal
      v-model:show="showCreate"
      :users="userNames"
      @created="onCreated"
    />
    <ProxyKeyRotateModal
      :access-key-id="rotateKeyId"
      @close="rotateKeyId = null"
      @rotated="onRotated"
    />
    <ProxySecretModal
      :secret="shownSecret"
      :kind="shownSecretKind"
      :endpoint="proxyAddress"
      @close="shownSecret = null"
    />
  </div>
</template>

<style lang="scss" scoped>
  .accounts-tile {
    &__note {
      margin: var(--tt-space-2) 0 var(--tt-space-4);
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }
  }
</style>
