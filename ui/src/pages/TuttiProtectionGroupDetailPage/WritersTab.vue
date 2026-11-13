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
  import { NButton, type DataTableColumns } from 'naive-ui';
  import { computed, h, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    Callout,
    ConfirmDialog,
    DataTable,
    EMPTY_VALUE,
    ErrorBlock,
  } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import { backendMessage } from '@/stores/tuttiProtectionStore';
  import type { ProtectionGroupActions } from '@/utils/composables/useProtectionGroupActions';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionGroup,
    ProtectionWriters,
  } from '@/utils/types/protection';

  interface WriterRow {
    token: string;
    worker: boolean;
    owner: string;
    generation: number;
    instance: string;
    admittedAt: Date | null;
  }

  const props = defineProps<{
    group: ProtectionGroup;
    writers: ProtectionWriters;
    writersError: boolean;
    actions: Pick<ProtectionGroupActions, 'clearWriter'>;
  }>();

  const emit = defineEmits<{ (e: 'retry'): void }>();

  const { t } = useI18n({ messages: i18nProtection });

  const rows = computed<WriterRow[]>(() => [
    ...Object.entries(props.writers.writers ?? {}).map(([token, record]) => ({
      token,
      worker: false,
      ...ProtectionGroupHelper.parseWriter(record),
    })),
    ...Object.entries(props.writers.workers ?? {}).map(([token, record]) => ({
      token,
      worker: true,
      ...ProtectionGroupHelper.parseWriter(record),
    })),
  ]);

  const clearing = ref<WriterRow | null>(null);
  const busy = ref(false);
  const error = ref<string | null>(null);

  function openClear(row: WriterRow) {
    error.value = null;
    clearing.value = row;
  }

  async function confirmClear() {
    const row = clearing.value;

    if (!row) return;

    busy.value = true;
    error.value = null;

    try {
      await props.actions.clearWriter(row.token);
      clearing.value = null;
    } catch (e) {
      error.value = t('clearFailed', {
        message: backendMessage(e) ?? 'unknown',
      });
    } finally {
      busy.value = false;
    }
  }

  const columns = computed<DataTableColumns<WriterRow>>(() => [
    { title: t('colWriterOwner'), key: 'owner' },
    {
      title: t('colWriterKind'),
      key: 'kind',
      render: (row) => (row.worker ? t('kindWorker') : t('kindProxy')),
    },
    {
      title: t('colWriterInstance'),
      key: 'instance',
      render: (row) =>
        h('span', { class: 'tt-mono' }, row.instance || EMPTY_VALUE),
    },
    {
      title: t('colWriterSince'),
      key: 'admittedAt',
      render: (row) => formatDateTime(row.admittedAt),
    },
    {
      title: '',
      key: 'actions',
      render: (row) =>
        h(WriteGate, null, {
          default: () =>
            h(
              NButton,
              {
                size: 'small',
                type: 'error',
                secondary: true,
                disabled: !props.group.frozen,
                title: props.group.frozen ? undefined : t('clearOnlyFrozen'),
                onClick: () => openClear(row),
              },
              { default: () => t('clearWriter') },
            ),
        }),
    },
  ]);
</script>

<template>
  <div class="writers-tab">
    <p class="writers-tab__help">{{ t('writersHelp') }}</p>
    <ErrorBlock
      v-if="writersError"
      :title="t('writersError')"
      @retry="emit('retry')"
    />
    <DataTable
      v-else
      :columns="columns"
      :rows="rows"
      :loading="false"
      :row-key="(r: WriterRow) => `${r.worker ? 'w' : 'p'}-${r.token}`"
    >
      <template #empty>
        <p class="writers-tab__help">{{ t('writersEmpty') }}</p>
      </template>
    </DataTable>

    <ConfirmDialog
      :open="clearing !== null"
      :title="t('clearTitle', { owner: clearing?.owner ?? '' })"
      :target-name="group.id"
      require-typing
      danger
      :loading="busy"
      :confirm-label="t('clearConfirm')"
      @confirm="confirmClear"
      @cancel="clearing = null"
    >
      <p>{{ t('clearText') }}</p>
      <p>{{ t('clearAttest') }}</p>
      <Callout
        v-if="error"
        tone="danger"
      >
        {{ error }}
      </Callout>
    </ConfirmDialog>
  </div>
</template>

<style lang="scss" scoped>
  .writers-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);

    &__help {
      max-width: var(--tt-measure);
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }
  }
</style>
