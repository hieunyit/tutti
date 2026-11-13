<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, h, ref } from 'vue';
  import { RouterLink } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput, NModal, type DataTableColumns } from 'naive-ui';
  import {
    Callout,
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    StatusBadge,
    type StatusTone,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import messages from '@/components/tutti/users/i18nAccess';
  import { TuttiService } from '@/services/TuttiService';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { RouteName } from '@/utils/types/router';
  import type { S3UserStorage } from '@/utils/types/users';

  const props = defineProps<{
    userName: string;
    storages: S3UserStorage[];
    dynamicEnabled: boolean;
  }>();
  const emit = defineEmits<{ (e: 'changed'): void }>();

  const { t } = useI18n({ messages });
  const busy = ref(false);
  const error = ref('');
  const results = ref<Record<string, string>>({});
  const editStorage = ref<string | null>(null);
  const removeStorage = ref<string | null>(null);
  const accessKey = ref('');
  /** typed secret: memory only, cleared on close */
  const secretKey = ref('');

  const tones: Record<string, StatusTone> = {
    'connection-account': 'success',
    override: 'info',
    missing: 'warning',
  };
  const modeLabels: Record<string, string> = {
    'connection-account': 'modeConnectionLabel',
    override: 'modeOverrideLabel',
    missing: 'modeMissingLabel',
  };

  function openEdit(storage: string) {
    editStorage.value = storage;
    accessKey.value = '';
    secretKey.value = '';
    error.value = '';
  }

  function closeEdit() {
    editStorage.value = null;
    secretKey.value = '';
  }

  async function guarded(task: () => Promise<void>) {
    busy.value = true;
    error.value = '';

    try {
      await task();

      return true;
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');

      return false;
    } finally {
      busy.value = false;
    }
  }

  async function save() {
    if (!editStorage.value || !accessKey.value || !secretKey.value) return;

    const ok = await guarded(async () => {
      await TuttiService.updateUserCredentials({
        name: props.userName,
        storage: editStorage.value as string,
        accessKeyId: accessKey.value,
        secretAccessKey: secretKey.value,
        verify: true,
      });
    });

    if (ok) {
      closeEdit();
      emit('changed');
    }
  }

  async function removeOverride() {
    const storage = removeStorage.value;

    if (!storage) return;

    const ok = await guarded(() =>
      TuttiService.removeUserCredential(props.userName, storage),
    );

    if (ok) {
      removeStorage.value = null;
      emit('changed');
    }
  }

  async function test(storage: string) {
    busy.value = true;

    try {
      const result = await TuttiService.testUser({
        name: props.userName,
        storage,
      });

      results.value[storage] = result.ok
        ? t('verified', { latency: result.latencyMs ?? 0 })
        : (result.error ?? t('failed'));
    } catch (e) {
      results.value[storage] = ProxyAccountHelper.getMessage(e) ?? t('failed');
    } finally {
      busy.value = false;
    }
  }

  const columns = computed<DataTableColumns<S3UserStorage>>(() => [
    {
      title: t('storages'),
      key: 'storage',
      render: (entry) =>
        h(
          RouterLink,
          {
            to: {
              name: RouteName.CHORUS_STORAGE_DETAILS,
              params: { storageName: entry.storage },
            },
          },
          () => entry.storage,
        ),
    },
    {
      title: t('mode'),
      key: 'mode',
      render: (entry) => {
        const mode = UserHelper.modeOf(entry);

        return h('div', [
          h(StatusBadge, {
            tone: tones[mode] ?? 'warning',
            label: t(modeLabels[mode] ?? 'modeMissingLabel'),
          }),
          results.value[entry.storage]
            ? h(
                'div',
                { class: 'user-cred__result' },
                results.value[entry.storage],
              )
            : null,
        ]);
      },
    },
    {
      title: t('accessKeyId'),
      key: 'accessKeyId',
      render: (entry) => h('code', entry.accessKeyId || '–'),
    },
    {
      title: t('source'),
      key: 'source',
      render: (entry) => t(entry.source === 'config' ? 'config' : 'managed'),
    },
    {
      title: t('actions'),
      key: 'actions',
      render: (entry) => {
        const mode = UserHelper.modeOf(entry);
        const isOverride = mode === 'override' && entry.source !== 'config';

        return h(WriteGate, null, () =>
          h('div', { class: 'user-cred__actions' }, [
            h(
              NButton,
              {
                size: 'small',
                disabled: busy.value || mode === 'missing',
                onClick: () => test(entry.storage),
              },
              () => t('test'),
            ),
            h(
              NButton,
              {
                size: 'small',
                disabled: entry.source === 'config' || !props.dynamicEnabled,
                onClick: () => openEdit(entry.storage),
              },
              () => t(mode === 'override' ? 'update' : 'add'),
            ),
            isOverride
              ? h(
                  NButton,
                  {
                    size: 'small',
                    secondary: true,
                    type: 'error',
                    disabled: busy.value || !props.dynamicEnabled,
                    onClick: () => {
                      error.value = '';
                      removeStorage.value = entry.storage;
                    },
                  },
                  () => t('removeOverride'),
                )
              : null,
          ]),
        );
      },
    },
  ]);
</script>

<template>
  <div>
    <p class="user-cred__hint">{{ t('credentialHint') }}</p>
    <DataTable
      :columns="columns"
      :rows="storages"
      :loading="false"
      :row-key="(entry: S3UserStorage) => entry.storage"
    >
      <template #empty>
        <EmptyBlock
          :title="t('noCredentialsTitle')"
          :description="t('noCredentialsText')"
        />
      </template>
    </DataTable>

    <NModal
      :show="editStorage !== null"
      preset="card"
      :title="t('add')"
      style="width: 560px; max-width: 94vw"
      @update:show="!$event && closeEdit()"
    >
      <form
        class="user-cred__form"
        @submit.prevent="save"
      >
        <Callout
          v-if="error"
          tone="danger"
          >{{ error }}</Callout
        >
        <label class="user-cred__field">
          {{ t('accessKeyId') }}
          <NInput
            v-model:value="accessKey"
            :input-props="{ autocomplete: 'off' }"
          />
        </label>
        <label class="user-cred__field">
          {{ t('secret') }}
          <NInput
            v-model:value="secretKey"
            type="password"
            :input-props="{ autocomplete: 'new-password' }"
          />
        </label>
        <NButton
          type="primary"
          attr-type="submit"
          :loading="busy"
          :disabled="!accessKey || !secretKey"
          >{{ t('verifyAndSave') }}</NButton
        >
      </form>
    </NModal>

    <ConfirmDialog
      :open="removeStorage !== null"
      :title="t('removeOverride')"
      :target-name="removeStorage ?? undefined"
      :confirm-label="t('removeOverride')"
      :loading="busy"
      danger
      @cancel="removeStorage = null"
      @confirm="removeOverride"
    >
      {{
        t('removeOverrideConfirm', {
          name: userName,
          storage: removeStorage ?? '',
        })
      }}
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
    </ConfirmDialog>
  </div>
</template>

<style lang="scss">
  .user-cred {
    &__hint {
      margin: 0 0 var(--tt-space-4);
      max-width: var(--tt-measure);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__result {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }

    &__actions {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }

    &__form {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-3);
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      font-size: var(--tt-text-sm);
    }
  }
</style>
