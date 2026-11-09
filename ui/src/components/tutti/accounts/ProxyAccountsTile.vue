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
  import AccessPolicyDrawer from './AccessPolicyDrawer.vue';
  import ProxyAccountCreateModal from './ProxyAccountCreateModal.vue';
  import ProxyAccountKeysTable from './ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from './ProxyKeyRotateModal.vue';
  import ProxySecretModal from './ProxySecretModal.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import type { StatusVariant } from '@/components/ui/status';
  import ConfirmDialog from '@/components/tutti/common/ConfirmDialog.vue';
  import FilterBar from '@/components/tutti/common/FilterBar.vue';
  import SearchInput from '@/components/tutti/common/SearchInput.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { IconName } from '@/utils/types/icon';
  import {
    TuttiAccessPermission,
    type TuttiProxyAccount,
    type TuttiProxyAccountSecret,
  } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { accounts, proxyAddress } = storeToRefs(store);
  const { createNotification } = useChorusNotification();

  const query = ref('');
  const expanded = ref<Set<string>>(new Set());
  const showDrawer = ref(false);
  const editedAccount = ref<TuttiProxyAccount | null>(null);
  const removing = ref<TuttiProxyAccount | null>(null);

  function openEdit(account: TuttiProxyAccount) {
    editedAccount.value = account;
    showDrawer.value = true;
  }

  async function confirmRemove() {
    const account = removing.value;

    if (!account) return;

    if (await store.removePolicy(account.user)) {
      removing.value = null;

      return;
    }

    createNotification({
      type: 'error',
      title: t('removeFailed'),
      duration: 6000,
    });
  }

  function toggle(user: string) {
    const next = new Set(expanded.value);

    if (next.has(user)) next.delete(user);
    else next.add(user);

    expanded.value = next;
  }

  const permissionMeta = computed<
    Record<string, { variant: StatusVariant; label: string }>
  >(() => ({
    [TuttiAccessPermission.READ_WRITE]: {
      variant: 'success',
      label: t('permissionReadWrite'),
    },
    [TuttiAccessPermission.READ_ONLY]: {
      variant: 'info',
      label: t('permissionReadOnly'),
    },
    [TuttiAccessPermission.WRITE_ONLY]: {
      variant: 'warning',
      label: t('permissionWriteOnly'),
    },
  }));

  function permissionOf(account: TuttiProxyAccount) {
    return (
      permissionMeta.value[
        account.policy?.permission ?? TuttiAccessPermission.READ_WRITE
      ] ?? permissionMeta.value[TuttiAccessPermission.READ_WRITE]!
    );
  }

  const showCreate = ref(false);
  const rotateKeyId = ref<string | null>(null);
  /** one-time secret being displayed; cleared on close, never persisted */
  const shownSecret = ref<TuttiProxyAccountSecret | null>(null);
  const shownSecretKind = ref<'created' | 'rotated'>('created');

  const userNames = computed(() => accounts.value.map((a) => a.user));

  const visibleAccounts = computed(() => {
    const needle = query.value.trim().toLowerCase();

    if (!needle) return accounts.value;

    return accounts.value.filter(
      (account) =>
        account.user.toLowerCase().includes(needle) ||
        (account.accessKeyIds ?? []).some((id) =>
          id.toLowerCase().includes(needle),
        ),
    );
  });

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

  function keysSummary(account: TuttiProxyAccount): string {
    const keys = ProxyAccountHelper.getKeyRows(account);
    const managed = keys.filter((key) => key.dynamicKey).length;
    const total = t('keysCount', { count: keys.length }, keys.length);

    return managed > 0
      ? t('keysSummaryManaged', { keys: total, managed })
      : t('keysSummaryConfig', { keys: total });
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
      v-if="accounts.length === 0"
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

    <div
      v-else
      class="tt-table-wrap"
    >
      <table class="tt-table">
        <thead>
          <tr>
            <th
              scope="col"
              class="accounts-tile__expand"
            >
              <span class="tt-sr-only">{{ t('expandColumn') }}</span>
            </th>
            <th scope="col">{{ t('columnStorageUser') }}</th>
            <th scope="col">{{ t('columnPermission') }}</th>
            <th scope="col">{{ t('columnBuckets') }}</th>
            <th scope="col">{{ t('columnAccessKeys') }}</th>
            <th scope="col">
              <span class="tt-sr-only">{{ t('columnActions') }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <template
            v-for="account in visibleAccounts"
            :key="account.user"
          >
            <tr>
              <td class="accounts-tile__expand">
                <button
                  type="button"
                  class="accounts-tile__toggle"
                  :aria-expanded="expanded.has(account.user)"
                  :aria-label="
                    expanded.has(account.user)
                      ? t('collapse', { user: account.user })
                      : t('expand', { user: account.user })
                  "
                  @click="toggle(account.user)"
                >
                  {{ expanded.has(account.user) ? '▾' : '▸' }}
                </button>
              </td>
              <td>
                <strong>{{ account.user }}</strong>
              </td>
              <td>
                <StatusTag
                  :variant="permissionOf(account).variant"
                  :label="permissionOf(account).label"
                  no-dot
                />
              </td>
              <td>
                <template
                  v-if="
                    AccessPolicyHelper.isAllBuckets(account.policy?.buckets)
                  "
                >
                  {{ t('allBucketsNoPolicy') }}
                  <span
                    v-if="!account.hasPolicy"
                    class="accounts-tile__muted"
                    >{{ t('noPolicyShort') }}</span
                  >
                </template>
                <span
                  v-else
                  class="accounts-tile__chips"
                >
                  <span
                    v-for="bucket in account.policy?.buckets ?? []"
                    :key="bucket"
                    class="accounts-tile__chip"
                    >{{ bucket }}</span
                  >
                </span>
              </td>
              <td>{{ keysSummary(account) }}</td>
              <td class="accounts-tile__actions">
                <WriteGate>
                  <span class="accounts-tile__buttons">
                    <CButton
                      secondary
                      size="small"
                      @click="openEdit(account)"
                    >
                      {{
                        account.hasPolicy
                          ? t('editPermissions')
                          : t('setPermissions')
                      }}
                    </CButton>
                    <CButton
                      v-if="account.hasPolicy"
                      type="error"
                      ghost
                      size="small"
                      @click="removing = account"
                    >
                      {{ t('removePolicyAction') }}
                    </CButton>
                  </span>
                </WriteGate>
              </td>
            </tr>
            <tr
              v-if="expanded.has(account.user)"
              class="accounts-tile__nested"
            >
              <td colspan="6">
                <div class="accounts-tile__nested-inner">
                  <ProxyAccountKeysTable
                    :account="account"
                    @rotate="rotateKeyId = $event"
                  />
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

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

    <AccessPolicyDrawer
      v-model:show="showDrawer"
      :account="editedAccount"
    />

    <ConfirmDialog
      :show="removing !== null"
      :title="t('removePolicyAction')"
      :text="t('removeConfirm', { user: removing?.user ?? '' })"
      :confirm-label="t('removeConfirmAction')"
      :cancel-label="t('cancel')"
      danger
      @update:show="removing = null"
      @confirm="confirmRemove"
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
