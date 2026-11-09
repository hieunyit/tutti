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
  import { storeToRefs } from 'pinia';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import ProxyAccountCreateModal from './ProxyAccountCreateModal.vue';
  import ProxyAccountKeysTable from './ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from './ProxyKeyRotateModal.vue';
  import ProxySecretModal from './ProxySecretModal.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import FilterBar from '@/components/tutti/common/FilterBar.vue';
  import SearchInput from '@/components/tutti/common/SearchInput.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { IconName } from '@/utils/types/icon';
  import { type TuttiProxyAccountSecret } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { accounts, proxyAddress } = storeToRefs(store);
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

  // the primary "Create access key" action lives in the page header
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
  <SectionCard
    class="accounts-tile"
    :title="t('title')"
    flush
  >
    <FilterBar>
      <SearchInput
        v-model="query"
        :placeholder="t('filterPlaceholder')"
        :label="t('filterLabel')"
      />
      <template #note>
        {{ t('filterNote') }}
      </template>
    </FilterBar>

    <EmptyState
      v-if="
        accounts.length === 0 ||
        accounts.every((account) => (account.accessKeyIds ?? []).length === 0)
      "
      :icon="IconName.BASE_KEY"
      :title="t('emptyTitle')"
      :text="t('emptyText')"
    />
    <EmptyState
      v-else-if="visibleAccounts.length === 0"
      :title="t('noResultsTitle')"
      :text="t('noResultsText')"
    >
      <CButton @click="query = ''">{{ t('noResultsAction') }}</CButton>
    </EmptyState>

    <ProxyAccountKeysTable
      v-else
      :accounts="visibleAccounts"
      @rotate="rotateKeyId = $event"
    />

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
  </SectionCard>
</template>

<style lang="scss" scoped>
  .accounts-tile {
    strong {
      font-weight: 600;
    }

    &__expand {
      width: 32px;
      padding-right: 0;
    }

    &__toggle {
      width: 28px;
      height: 28px;
      padding: 0;
      font: inherit;
      color: var(--tt-text-2);
      background: var(--tt-surface);
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
      cursor: pointer;

      &:hover {
        background: var(--tt-subtle);
      }
    }

    &__muted {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__chips {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    &__chip {
      font-family: var(--tt-mono);
      font-size: 12px;
    }

    // width 0 + min-width 100%: the nested table scrolls inside the cell and
    // never widens the users table
    &__nested-inner {
      width: 0;
      min-width: 100%;
    }

    &__nested td {
      padding: 4px 16px 16px 40px;
      background-color: var(--tt-subtle);
    }

    &__actions {
      text-align: right;
    }

    &__buttons {
      display: inline-flex;
      gap: 6px;
    }
  }
</style>
