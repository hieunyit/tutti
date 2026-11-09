<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import {
    NButton,
    NForm,
    NFormItem,
    NInput,
    NModal,
    NSteps,
    NStep,
  } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import i18nStorages from './i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import type {
    RuntimeStorageCredential,
    RuntimeStorageTestResult,
  } from '@/utils/types/runtimeStorage';

  const props = defineProps<{ show: boolean; users: string[] }>();
  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'created', name: string): void;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const storage = ref(RuntimeStorageHelper.empty());
  const credentials = ref<RuntimeStorageCredential[]>([]);
  const results = ref<Record<string, RuntimeStorageTestResult>>({});
  const step = ref(1);
  const busy = ref(false);
  const error = ref('');
  const tested = ref('');
  const supplied = computed(() =>
    credentials.value.filter(
      (item) => item.accessKeyId || item.secretAccessKey,
    ),
  );
  const complete = computed(
    () =>
      supplied.value.length > 0 &&
      supplied.value.every(
        (item) => item.accessKeyId.trim() && item.secretAccessKey,
      ),
  );
  const fingerprint = computed(() =>
    JSON.stringify([
      RuntimeStorageHelper.payload(storage.value),
      supplied.value,
    ]),
  );
  const canSave = computed(
    () => complete.value && tested.value === fingerprint.value,
  );

  watch(
    () => props.show,
    (show) => {
      if (!show) {
        credentials.value = [];
        results.value = {};
        tested.value = '';

        return;
      }

      storage.value = RuntimeStorageHelper.empty();
      credentials.value = props.users.map((user) => ({
        user,
        accessKeyId: '',
        secretAccessKey: '',
      }));
      step.value = 1;
      error.value = '';
      tested.value = '';
      results.value = {};
    },
  );
  watch(fingerprint, () => {
    tested.value = '';
    results.value = {};
  });

  function next() {
    error.value = '';

    if (!RuntimeStorageHelper.validConnection(storage.value)) {
      error.value = t('invalid');

      return;
    }

    step.value = 2;
  }

  async function test() {
    if (busy.value) return;

    error.value = '';

    if (!complete.value) {
      error.value = t('incomplete');

      return;
    }

    busy.value = true;
    const version = fingerprint.value;
    const output: Record<string, RuntimeStorageTestResult> = {};

    try {
      for (const credential of supplied.value) {
        output[credential.user] = await RuntimeStorageService.test(
          RuntimeStorageHelper.payload(storage.value),
          credential,
        );
      }

      if (version !== fingerprint.value) return;

      results.value = output;

      if (Object.values(output).every((result) => result.ok))
        tested.value = version;
    } catch (e) {
      error.value = RuntimeStorageHelper.error(e, t('testFailed'));
    } finally {
      busy.value = false;
    }
  }

  async function save() {
    if (busy.value || !canSave.value) return;

    busy.value = true;
    error.value = '';

    try {
      const created = await RuntimeStorageService.create({
        storage: RuntimeStorageHelper.payload(storage.value),
        credentials: supplied.value,
      });

      emit('created', created.name);
      emit('update:show', false);
      credentials.value = [];
    } catch (e) {
      error.value = RuntimeStorageHelper.error(e, t('error'));
    } finally {
      busy.value = false;
    }
  }
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('add')"
    :closable="!busy"
    :mask-closable="!busy"
    :style="{ width: 'min(680px, calc(100vw - 32px))' }"
    @update:show="emit('update:show', $event)"
  >
    <p class="storage-dialog__sub">{{ t('propagation') }}</p>
    <NSteps
      :current="step"
      size="small"
      ><NStep :title="t('connection')" /><NStep :title="t('credentials')"
    /></NSteps>
    <NForm
      class="storage-dialog__form"
      label-placement="top"
      @submit.prevent
    >
      <StorageConnectionForm
        v-if="step === 1"
        v-model="storage"
        show-name
        :readonly="busy"
      />
      <template v-else>
        <p class="storage-dialog__sub">{{ t('credentialsHint') }}</p>
        <Callout
          v-if="users.length === 0"
          tone="warning"
          >{{ t('noUsers') }}</Callout
        >
        <fieldset
          v-for="credential in credentials"
          :key="credential.user"
          class="storage-dialog__credential"
          :disabled="busy"
        >
          <legend>{{ credential.user }}</legend>
          <div class="storage-dialog__pair">
            <NFormItem :label="t('accessKey')"
              ><NInput
                v-model:value="credential.accessKeyId"
                :placeholder="t('skip')"
                :disabled="busy"
                autocomplete="off" /></NFormItem
            ><NFormItem :label="t('secret')"
              ><NInput
                v-model:value="credential.secretAccessKey"
                type="password"
                show-password-on="click"
                :disabled="busy"
                autocomplete="new-password"
            /></NFormItem>
          </div>
          <p
            v-if="results[credential.user]"
            role="status"
            :class="
              results[credential.user]?.ok
                ? 'storage-dialog__ok'
                : 'storage-dialog__failed'
            "
          >
            {{
              results[credential.user]?.ok
                ? t('testOk', { ms: results[credential.user]?.latencyMs ?? 0 })
                : results[credential.user]?.error || t('testFailed')
            }}
          </p>
          <p
            v-else-if="!credential.accessKeyId && !credential.secretAccessKey"
            class="storage-dialog__sub"
          >
            {{ t('skipped') }}
          </p>
        </fieldset>
      </template>
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
    </NForm>
    <template #footer
      ><div class="storage-dialog__actions">
        <NButton
          :disabled="busy"
          @click="emit('update:show', false)"
          >{{ t('cancel') }}</NButton
        >
        <div class="storage-dialog__buttons">
          <NButton
            v-if="step === 2"
            :disabled="busy"
            @click="step = 1"
            >{{ t('back') }}</NButton
          ><NButton
            v-if="step === 1"
            type="primary"
            @click="next"
            >{{ t('next') }}</NButton
          ><template v-else
            ><NButton
              :loading="busy"
              :disabled="!complete"
              @click="test"
              >{{ t('test') }}</NButton
            ><NButton
              type="primary"
              :loading="busy"
              :disabled="!canSave"
              @click="save"
              >{{ t('add') }}</NButton
            ></template
          >
        </div>
      </div></template
    >
  </NModal>
</template>

<style scoped lang="scss">
  .storage-dialog {
    &__sub {
      color: var(--tt-text-2);
      font-size: 13px;
    }

    &__form {
      margin-top: 20px;
    }

    &__credential {
      min-width: 0;
      margin: 12px 0;
      padding: 12px;
      border: 1px solid var(--tt-border);
      border-radius: 8px;
    }

    &__pair {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }

    &__actions,
    &__buttons {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: space-between;
    }

    &__ok {
      color: var(--tt-success);
    }

    &__failed {
      color: var(--tt-danger);
      overflow-wrap: anywhere;
    }
    @media (max-width: 600px) {
      &__pair {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
