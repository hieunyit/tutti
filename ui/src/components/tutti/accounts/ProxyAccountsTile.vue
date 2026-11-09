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
  import { CTile } from '@clyso/clyso-ui-kit';
  import {
    NAlert,
    NButton,
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
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import {
    TuttiAccessPermission,
    type TuttiProxyAccount,
  } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { accounts } = storeToRefs(store);
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

  function sourceLabel(source: string): string {
    return source === 'custom' ? t('sourceCustom') : source;
  }

  const columns = computed<DataTableColumns<TuttiProxyAccount>>(() => [
    { title: t('columnUser'), key: 'user' },
    {
      title: t('columnKeys'),
      key: 'keys',
      render: (row) =>
        h(NSpace, { size: 4 }, () =>
          AccessPolicyHelper.getAccessKeys(row).map((key) =>
            h(
              NSpace,
              { size: 2, key: key.accessKeyId, align: 'center' },
              () => [
                h('code', key.accessKeyId),
                h(NTag, { size: 'tiny', round: true }, () =>
                  sourceLabel(key.source),
                ),
              ],
            ),
          ),
        ),
    },
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
      title: t('columnActions'),
      key: 'actions',
      render: (row) =>
        h(WriteGate, null, () =>
          h(NSpace, { size: 8 }, () => [
            h(NButton, { size: 'small', onClick: () => openEdit(row) }, () =>
              row.hasPolicy ? t('editPolicy') : t('createPolicy'),
            ),
            row.hasPolicy
              ? h(
                  NPopconfirm,
                  {
                    positiveText: t('removeConfirmAction'),
                    negativeText: t('cancel'),
                    onPositiveClick: () => remove(row),
                  },
                  {
                    trigger: () =>
                      h(NButton, { size: 'small', type: 'error' }, () =>
                        t('removePolicy'),
                      ),
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
    <template #title>
      {{ t('title') }}
    </template>

    <NAlert
      type="info"
      class="accounts-tile__banner"
    >
      {{ t('banner') }}
    </NAlert>

    <NDataTable
      :columns="columns"
      :data="accounts"
      :bordered="false"
      :row-key="(row: TuttiProxyAccount) => row.user"
    >
      <template #empty>{{ t('noAccounts') }}</template>
    </NDataTable>

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

    :deep(.accounts-tile__hint) {
      font-size: 12px;
      opacity: 0.7;
    }
  }
</style>
