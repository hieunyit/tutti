<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NCheckbox, NModal, NSpin } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageAccountForm from './StorageAccountForm.vue';
  import StorageTestSteps from './StorageTestSteps.vue';
  import i18nStorages from './i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
  import type {
    RuntimeStorage,
    StorageConnectionAccount,
    StorageTestResult,
  } from '@/utils/types/runtimeStorage';

  const props = defineProps<{
    show: boolean;
    storage: RuntimeStorage;
    /** rotate replaces a working account, set adds the first one */
    mode: 'rotate' | 'set';
  }>();
  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'done'): void;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const account = ref<StorageConnectionAccount>({
    accessKeyId: '',
    secretAccessKey: '',
  });
  const options = ref(StorageTestHelper.emptyOptions());
  const result = ref<StorageTestResult | null>(null);
  const testing = ref(false);
  const busy = ref(false);
  const error = ref('');
  const saveAnyway = ref(false);
  const confirmed = ref(false);
  const complete = computed(
    () =>
      account.value.accessKeyId.trim() !== '' &&
      account.value.secretAccessKey !== '',
  );
  const canApply = computed(() =>
    StorageTestHelper.canSave(result.value, saveAnyway.value, confirmed.value),
  );
  const gate = computed(() =>
    StorageTestHelper.saveGate(result.value, saveAnyway.value),
  );
  let generation = 0;

  function reset() {
    generation += 1;
    account.value = { accessKeyId: '', secretAccessKey: '' };
    options.value = StorageTestHelper.emptyOptions();
    result.value = null;
    error.value = '';
    saveAnyway.value = false;
    confirmed.value = false;
    testing.value = false;
    busy.value = false;
  }

  watch(() => props.show, reset);
  watch([account, options], () => {
    generation += 1;
    result.value = null;
    saveAnyway.value = false;
    confirmed.value = false;
    testing.value = false;
  });
  watch(saveAnyway, (value) => {
    if (!value) confirmed.value = false;
  });

  async function runTest() {
    if (!complete.value || testing.value || busy.value) return;

    error.value = '';
    testing.value = true;

    const current = generation;

    try {
      const response = await RuntimeStorageService.test({
        name: props.storage.name,
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

  async function apply() {
    if (busy.value || !canApply.value) return;

    busy.value = true;
    error.value = '';

    const force = StorageTestHelper.forceFlag(
      result.value,
      saveAnyway.value,
      confirmed.value,
    );
    const testOptions = StorageTestHelper.toOptions(options.value);

    try {
      if (props.mode === 'rotate')
        await RuntimeStorageService.rotate({
          name: props.storage.name,
          newAccount: account.value,
          options: testOptions,
          force,
        });
      else
        await RuntimeStorageService.update({
          storage: RuntimeStorageHelper.payload(props.storage),
          connectionAccount: account.value,
          testOptions,
          force,
        });

      emit('done');
      emit('update:show', false);
      reset();
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
    :title="mode === 'rotate' ? t('rotate') : t('setAccount')"
    :closable="!busy"
    :mask-closable="!busy"
    :style="{ width: 'min(760px, calc(100vw - 32px))' }"
    @update:show="emit('update:show', $event)"
  >
    <div class="rotate-dialog">
      <p class="rotate-dialog__sub">{{ t('rotateHint') }}</p>
      <StorageAccountForm
        v-model:account="account"
        v-model:options="options"
        :provider="storage.provider"
        :readonly="busy"
      />
      <div
        v-if="testing"
        class="rotate-dialog__busy"
        role="status"
      >
        <NSpin size="small" />{{ t('testing') }}
      </div>
      <StorageTestSteps
        v-else-if="result"
        :result="result"
      />
      <template v-if="result && gate !== 'ok'">
        <Callout tone="danger">{{
          mode === 'rotate' ? t('rotateFailed') : t('saveBlocked')
        }}</Callout>
        <NCheckbox
          v-model:checked="saveAnyway"
          :disabled="busy"
          >{{
            mode === 'rotate' ? t('switchAnyway') : t('saveAnyway')
          }}</NCheckbox
        >
        <NCheckbox
          v-if="saveAnyway"
          v-model:checked="confirmed"
          :disabled="busy"
          >{{ t('saveAnywayConfirm') }}</NCheckbox
        >
      </template>
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
    </div>
    <template #footer
      ><div class="rotate-dialog__actions">
        <NButton
          :disabled="busy"
          @click="emit('update:show', false)"
          >{{ t('cancel') }}</NButton
        >
        <div class="rotate-dialog__actions">
          <NButton
            :loading="testing"
            :disabled="!complete || busy"
            @click="runTest"
            >{{ result ? t('retest') : t('runTest') }}</NButton
          ><NButton
            type="primary"
            :loading="busy"
            :disabled="!canApply"
            @click="apply"
            >{{
              mode === 'rotate' ? t('rotateAction') : t('setAccount')
            }}</NButton
          >
        </div>
      </div></template
    >
  </NModal>
</template>

<style scoped lang="scss">
  .rotate-dialog {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;

    &__sub {
      margin: 0;
      color: var(--tt-text-2);
      font-size: 13px;
    }

    &__busy {
      display: flex;
      align-items: center;
      gap: 10px;
      color: var(--tt-text-2);
    }

    &__actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: space-between;
    }
  }
</style>
