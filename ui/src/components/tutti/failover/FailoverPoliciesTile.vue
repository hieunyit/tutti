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
    NDataTable,
    NPopconfirm,
    NSpace,
    NTag,
    type DataTableColumns,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from './i18nFailover';
  import FailoverPolicyDrawer from './FailoverPolicyDrawer.vue';
  import RowActionButton from '@/components/tutti/common/RowActionButton.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { IconName } from '@/utils/types/icon';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import type { TuttiFailoverPolicy } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { policies } = storeToRefs(store);
  const { createNotification } = useChorusNotification();

  const showDrawer = ref(false);
  const editedPolicy = ref<TuttiFailoverPolicy | null>(null);

  function openAdd() {
    editedPolicy.value = null;
    showDrawer.value = true;
  }

  // the primary "Add policy" action lives in the page header
  defineExpose({ openAdd });

  function openEdit(policy: TuttiFailoverPolicy) {
    editedPolicy.value = policy;
    showDrawer.value = true;
  }

  async function remove(policy: TuttiFailoverPolicy) {
    if (!(await store.deletePolicy(policy.source))) {
      createNotification({
        type: 'error',
        title: t('deleteFailed'),
        duration: 6000,
      });
    }
  }

  const columns = computed<DataTableColumns<TuttiFailoverPolicy>>(() => [
    { title: t('columnSource'), key: 'source' },
    {
      title: t('columnEnabled'),
      key: 'enabled',
      render: (row) =>
        h(
          NTag,
          { type: row.enabled ? 'success' : 'warning', size: 'small' },
          () => (row.enabled ? t('statusOn') : t('statusOff')),
        ),
    },
    {
      title: t('columnTargets'),
      key: 'targets',
      render: (row) =>
        h(NSpace, { size: 4 }, () =>
          (row.targets ?? []).map((target, index) =>
            h(
              NTag,
              { size: 'small', key: target },
              () => `${index + 1}. ${target}`,
            ),
          ),
        ),
    },
    {
      title: t('columnAfter'),
      key: 'after',
      render: (row) => row.after ?? t('defaultValue'),
    },
    {
      title: t('columnMaxEventLag'),
      key: 'maxEventLag',
      render: (row) => String(row.maxEventLag ?? t('defaultValue')),
    },
    {
      title: t('columnActions'),
      key: 'actions',
      render: (row) =>
        h(WriteGate, null, () =>
          h('span', { class: 'failover-policies-tile__actions' }, [
            h(RowActionButton, {
              label: t('edit'),
              icon: IconName.BASE_CREATE,
              type: 'primary',
              onClick: () => openEdit(row),
            }),
            h(
              NPopconfirm,
              {
                positiveText: t('deleteConfirmAction'),
                negativeText: t('cancel'),
                positiveButtonProps: { type: 'error' },
                onPositiveClick: () => remove(row),
              },
              {
                trigger: () =>
                  h(RowActionButton, {
                    label: t('delete'),
                    icon: IconName.BASE_TRASH,
                    type: 'error',
                  }),
                default: () => t('deleteConfirm', { source: row.source }),
              },
            ),
          ]),
        ),
    },
  ]);
</script>

<template>
  <CTile class="failover-policies-tile">
    <template #title>
      {{ t('policiesTitle') }}
    </template>

    <NDataTable
      :columns="columns"
      :data="policies"
      :bordered="false"
      :scroll-x="760"
      :row-key="(row: TuttiFailoverPolicy) => row.source"
    >
      <template #empty>
        <CResult
          class="failover-policies-tile__empty"
          type="empty"
          size="medium"
          :icon-name="IconName.BASE_REFRESH"
          :max-width="480"
        >
          <template #title>
            {{ t('policiesEmptyTitle') }}
          </template>

          <p>{{ t('policiesEmptyText') }}</p>

          <template #actions>
            <WriteGate>
              <CButton
                type="primary"
                @click="openAdd"
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
        </CResult>
      </template>
    </NDataTable>

    <FailoverPolicyDrawer
      v-model:show="showDrawer"
      :policy="editedPolicy"
    />
  </CTile>
</template>

<style lang="scss" scoped>
  .failover-policies-tile {
    &__empty {
      padding: 24px 0;
    }

    :deep(.failover-policies-tile__actions) {
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }
  }
</style>
