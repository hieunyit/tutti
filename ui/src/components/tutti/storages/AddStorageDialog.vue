<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import {
    NButton,
    NCheckbox,
    NForm,
    NModal,
    NSpin,
    NStep,
    NSteps,
  } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageAccountForm from './StorageAccountForm.vue';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import StorageTestSteps from './StorageTestSteps.vue';
  import i18nStorages from './i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
  import type {
    StorageConnectionAccount,
    StorageTestResult,
  } from '@/utils/types/runtimeStorage';

  const props = defineProps<{ show: boolean }>();
  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'created', name: string): void;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const storage = ref(RuntimeStorageHelper.empty());
  const account = ref<StorageConnectionAccount>({
    accessKeyId: '',
    secretAccessKey: '',
  });
  const options = ref(StorageTestHelper.emptyOptions());
  const result = ref<StorageTestResult | null>(null);
  const step = ref(1);
  const busy = ref(false);
  const testing = ref(false);
  const error = ref('');
  const saveAnyway = ref(false);
  const confirmed = ref(false);
  const fingerprint = computed(() =>
    JSON.stringify([
      RuntimeStorageHelper.payload(storage.value),
      account.value,
      StorageTestHelper.toOptions(options.value),
    ]),
  );
  const accountComplete = computed(
    () =>
      account.value.accessKeyId.trim() !== '' &&
      account.value.secretAccessKey !== '',
  );
  const gate = computed(() =>
    StorageTestHelper.saveGate(result.value, saveAnyway.value),
  );
  const canSave = computed(() =>
    StorageTestHelper.canSave(result.value, saveAnyway.value, confirmed.value),
  );
  const failing = computed(() => StorageTestHelper.counts(result.value).fail);
  let generation = 0;

  function reset() {
    generation += 1;
    storage.value = RuntimeStorageHelper.empty();
    account.value = { accessKeyId: '', secretAccessKey: '' };
    options.value = StorageTestHelper.emptyOptions();
    result.value = null;
    step.value = 1;
    error.value = '';
    saveAnyway.value = false;
    confirmed.value = false;
    testing.value = false;
    busy.value = false;
  }

  watch(
    () => props.show,
    (show) => onShow(show),
  );
  watch(fingerprint, () => {
    generation += 1;
    result.value = null;
    saveAnyway.value = false;
    confirmed.value = false;
    testing.value = false;
  });
  watch(saveAnyway, (value) => {
    if (!value) confirmed.value = false;
  });

  function onShow(show: boolean) {
    if (show) reset();
    else {
      // never keep a secret in memory after the dialog closed
      account.value = { accessKeyId: '', secretAccessKey: '' };
      result.value = null;
    }
  }

  function next() {
    error.value = '';

    if (
      step.value === 1 &&
      !RuntimeStorageHelper.validConnection(storage.value)
    ) {
      error.value = t('invalid');

      return;
    }

    if (step.value === 2 && !accountComplete.value) {
      error.value = t('accountRequired');

      return;
    }

    step.value += 1;

    if (step.value === 3 && !result.value) runTest();
  }

  async function runTest() {
    if (testing.value || busy.value) return;

    error.value = '';
    result.value = null;
    saveAnyway.value = false;
    confirmed.value = false;
    testing.value = true;

    const current = generation;

    try {
      const response = await RuntimeStorageService.test({
        storage: RuntimeStorageHelper.payload(storage.value),
        connectionAccount: account.value,
        options: StorageTestHelper.toOptions(options.value),
      });

      if (current === generation) result.value = response;
    } catch (e) {
      if (current === generation)
        error.value = RuntimeStorageHelper.error(e, t('testError'));
    } finally {
      if (current === generation) testing.value = false;
    }
  }

  async function save() {
    if (busy.value || !canSave.value) return;

    busy.value = true;
    error.value = '';

    try {
      const created = await RuntimeStorageService.create({
        storage: RuntimeStorageHelper.payload(storage.value),
        connectionAccount: account.value,
        testOptions: StorageTestHelper.toOptions(options.value),
        force: StorageTestHelper.forceFlag(
          result.value,
          saveAnyway.value,
          confirmed.value,
        ),
      });

      emit('created', created.name);
      emit('update:show', false);
      onShow(false);
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
    :style="{ width: 'min(820px, calc(100vw - 32px))' }"
    @update:show="emit('update:show', $event)"
  >
    <p class="storage-dialog__sub">{{ t('propagation') }}</p>
    <div class="storage-dialog__steps">
      <NSteps
        :current="step"
        size="small"
        ><NStep :title="t('connection')" /><NStep
          :title="t('accountStep')" /><NStep :title="t('testStep')" /><NStep
          :title="t('saveStep')"
      /></NSteps>
    </div>
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
      <StorageAccountForm
        v-else-if="step === 2"
        v-model:account="account"
        v-model:options="options"
        :provider="storage.provider"
        :readonly="busy"
      />
      <template v-else-if="step === 3">
        <p class="storage-dialog__sub">{{ t('testIntro') }}</p>
        <div
          v-if="testing"
          class="storage-dialog__busy"
          role="status"
        >
          <NSpin size="small" />{{ t('testing') }}
        </div>
        <StorageTestSteps
          v-else-if="result"
          :result="result"
        />
        <NButton
          v-if="!testing"
          class="storage-dialog__retry"
          @click="runTest"
          >{{ result ? t('retest') : t('runTest') }}</NButton
        >
      </template>
      <template v-else>
        <StorageTestSteps
          v-if="result && failing > 0"
          :result="result"
        />
        <Callout
          v-if="gate === 'ok'"
          :tone="result?.hasWarnings ? 'warning' : 'info'"
          >{{ result?.hasWarnings ? t('saveWarn') : t('saveOk') }}</Callout
        >
        <template v-else>
          <Callout tone="danger">{{ t('saveBlocked') }}</Callout>
          <NCheckbox
            v-model:checked="saveAnyway"
            :disabled="busy || !result"
            >{{ t('saveAnyway') }} · {{ t('saveAnywayHint') }}</NCheckbox
          >
          <NCheckbox
            v-if="saveAnyway"
            v-model:checked="confirmed"
            :disabled="busy"
            >{{ t('saveAnywayConfirm') }}</NCheckbox
          >
        </template>
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
            v-if="step > 1"
            :disabled="busy || testing"
            @click="step -= 1"
            >{{ t('back') }}</NButton
          ><NButton
            v-if="step < 4"
            type="primary"
            :disabled="testing || (step === 3 && !result)"
            @click="next"
            >{{ t('next') }}</NButton
          ><NButton
            v-else
            type="primary"
            :loading="busy"
            :disabled="!canSave"
            @click="save"
            >{{ t('add') }}</NButton
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

    &__steps {
      overflow-x: auto;
      padding-bottom: 4px;
    }

    &__form {
      display: flex;
      flex-direction: column;
      gap: 12px;
      margin-top: 20px;
    }

    &__busy {
      display: flex;
      align-items: center;
      gap: 10px;
      color: var(--tt-text-2);
    }

    &__retry {
      align-self: flex-start;
    }

    &__actions,
    &__buttons {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: space-between;
    }
  }
</style>
