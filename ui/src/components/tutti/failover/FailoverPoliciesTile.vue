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
  import i18nFailover from './i18nFailover';
  import FailoverPolicyDrawer from './FailoverPolicyDrawer.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
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
          { type: row.enabled ? 'success' : 'default', size: 'small' },
          () => (row.enabled ? 'on' : 'off'),
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
          h(NSpace, { size: 8 }, () => [
            h(NButton, { size: 'small', onClick: () => openEdit(row) }, () =>
              t('edit'),
            ),
            h(
              NPopconfirm,
              {
                positiveText: t('deleteConfirmAction'),
                negativeText: t('cancel'),
                onPositiveClick: () => remove(row),
              },
              {
                trigger: () =>
                  h(NButton, { size: 'small', type: 'error' }, () =>
                    t('delete'),
                  ),
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

    <p class="failover-policies-tile__help">{{ t('help') }}</p>

    <WriteGate>
      <NButton
        type="primary"
        class="failover-policies-tile__add"
        @click="openAdd"
      >
        {{ t('addPolicy') }}
      </NButton>
    </WriteGate>

    <NDataTable
      :columns="columns"
      :data="policies"
      :bordered="false"
      :row-key="(row: TuttiFailoverPolicy) => row.source"
    />

    <FailoverPolicyDrawer
      v-model:show="showDrawer"
      :policy="editedPolicy"
    />
  </CTile>
</template>

<style lang="scss" scoped>
  .failover-policies-tile {
    &__help {
      margin: 0 0 16px;
      opacity: 0.8;
    }

    &__add {
      margin-bottom: 16px;
    }
  }
</style>
