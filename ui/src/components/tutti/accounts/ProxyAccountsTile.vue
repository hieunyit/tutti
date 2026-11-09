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
  import { CButton, CIcon, CResult, CTile } from '@clyso/clyso-ui-kit';
  import {
    NAlert,
    NDataTable,
    NPopconfirm,
    NSpace,
    NTag,
    type DataTableColumns,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import AccessPolicyDrawer from './AccessPolicyDrawer.vue';
  import ProxyAccountCreateModal from './ProxyAccountCreateModal.vue';
  import ProxyAccountKeysTable from './ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from './ProxyKeyRotateModal.vue';
  import ProxySecretModal from './ProxySecretModal.vue';
  import RowActionButton from '@/components/tutti/common/RowActionButton.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
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
  const { accounts, isLoading } = storeToRefs(store);
  const { createNotification } = useChorusNotification();

  const showDrawer = ref(false);
  const editedAccount = ref<TuttiProxyAccount | null>(null);

  function openEdit(account: TuttiProxyAccount) {
    editedAccount.value = account;
    showDrawer.value = true;
  }

  async function remove(account: TuttiProxyAccount) {
    if (!(await store.removePolicy(account.user))) {
      createNotification({
        type: 'error',
        title: t('removeFailed'),
        duration: 6000,
      });
    }
  }

  const permissionMeta = computed(() => ({
    [TuttiAccessPermission.READ_WRITE]: {
      type: 'success' as const,
      label: t('permissionReadWrite'),
    },
    [TuttiAccessPermission.READ_ONLY]: {
      type: 'info' as const,
      label: t('permissionReadOnly'),
    },
    [TuttiAccessPermission.WRITE_ONLY]: {
      type: 'warning' as const,
      label: t('permissionWriteOnly'),
    },
  }));

  const showCreate = ref(false);
  const rotateKeyId = ref<string | null>(null);
  /** one-time secret being displayed; cleared on close, never persisted */
  const shownSecret = ref<TuttiProxyAccountSecret | null>(null);
  const shownSecretKind = ref<'created' | 'rotated'>('created');

  const userNames = computed(() => accounts.value.map((a) => a.user));

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

  function keysSummary(account: TuttiProxyAccount): string {
    const keys = ProxyAccountHelper.getKeyRows(account);
    const dynamic = keys.filter((key) => key.dynamicKey).length;
    const total = t('keysCount', { count: keys.length }, keys.length);

    return dynamic > 0
      ? `${total} (${t('keysDynamic', { count: dynamic })})`
      : total;
  }

  const columns = computed<DataTableColumns<TuttiProxyAccount>>(() => [
    {
      type: 'expand',
      expandable: (row) => ProxyAccountHelper.getKeyRows(row).length > 0,
      renderExpand: (row) =>
        h(ProxyAccountKeysTable, {
          account: row,
          onRotate: (accessKeyId: string) => {
            rotateKeyId.value = accessKeyId;
          },
        }),
    },
    { title: t('columnUser'), key: 'user' },
    {
      title: t('columnPermission'),
      key: 'permission',
      render: (row) => {
        const meta =
          permissionMeta.value[
            (row.policy?.permission as TuttiAccessPermission) ??
              TuttiAccessPermission.READ_WRITE
          ] ?? permissionMeta.value[TuttiAccessPermission.READ_WRITE];

        return h(NSpace, { size: 4, align: 'center' }, () => [
          h(NTag, { type: meta.type, size: 'small' }, () => meta.label),
          row.hasPolicy
            ? null
            : h('span', { class: 'accounts-tile__hint' }, t('noPolicyHint')),
        ]);
      },
    },
    {
      title: t('columnBuckets'),
      key: 'buckets',
      render: (row) =>
        AccessPolicyHelper.isAllBuckets(row.policy?.buckets)
          ? h(NTag, { size: 'small' }, () => t('allBuckets'))
          : h(NSpace, { size: 4 }, () =>
              (row.policy?.buckets ?? []).map((bucket) =>
                h(NTag, { size: 'small', key: bucket }, () => bucket),
              ),
            ),
    },
    {
      title: t('columnKeys'),
      key: 'keys',
      render: (row) => keysSummary(row),
    },
    {
      title: t('columnActions'),
      key: 'actions',
      render: (row) =>
        h(WriteGate, null, () =>
          h('span', { class: 'accounts-tile__actions' }, [
            h(RowActionButton, {
              label: row.hasPolicy ? t('editPolicy') : t('createPolicy'),
              icon: IconName.BASE_CREATE,
              type: 'primary',
              onClick: () => openEdit(row),
            }),
            row.hasPolicy
              ? h(
                  NPopconfirm,
                  {
                    positiveText: t('removeConfirmAction'),
                    negativeText: t('cancel'),
                    positiveButtonProps: { type: 'error' },
                    onPositiveClick: () => remove(row),
                  },
                  {
                    trigger: () =>
                      h(RowActionButton, {
                        label: t('removePolicy'),
                        icon: IconName.BASE_REMOVE_CIRCLE,
                        type: 'error',
                      }),
                    default: () => t('removeConfirm', { user: row.user }),
                  },
                )
              : null,
          ]),
        ),
    },
  ]);
</script>

<template>
  <CTile class="accounts-tile">
    <NAlert
      type="info"
      class="accounts-tile__banner"
    >
      {{ t('banner') }}
    </NAlert>

    <NDataTable
      :columns="columns"
      :data="accounts"
      :loading="isLoading"
      :bordered="false"
      :scroll-x="880"
      :row-key="(row: TuttiProxyAccount) => row.user"
    >
      <template #empty>
        <CResult
          class="accounts-tile__empty"
          type="empty"
          size="medium"
          :icon-name="IconName.BASE_KEY"
          :max-width="480"
        >
          <template #title>
            {{ t('emptyTitle') }}
          </template>

          <p>{{ t('emptyText') }}</p>

          <template #actions>
            <WriteGate>
              <CButton
                type="primary"
                @click="showCreate = true"
              >
                <template #icon>
                  <CIcon
                    :is-inline="true"
                    :name="IconName.BASE_ADD"
                  />
                </template>

                {{ t('createAccount') }}
              </CButton>
            </WriteGate>
          </template>
        </CResult>
      </template>
    </NDataTable>

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
      @close="shownSecret = null"
    />

    <AccessPolicyDrawer
      v-model:show="showDrawer"
      :account="editedAccount"
    />
  </CTile>
</template>

<style lang="scss" scoped>
  .accounts-tile {
    &__banner {
      margin-bottom: 16px;
    }

    &__empty {
      padding: 24px 0;
    }

    :deep(.accounts-tile__actions) {
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }

    :deep(.accounts-tile__hint) {
      font-size: 12px;
      color: var(--text-color-3);
    }
  }
</style>
