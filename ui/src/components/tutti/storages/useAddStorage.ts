/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import i18nStorages from './i18nStorages';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
import type {
  StorageConnectionAccount,
  StorageTestResult,
} from '@/utils/types/runtimeStorage';

/** State and steps of the add-storage dialog (connection, account, test, save). */
export function useAddStorage(
  props: { show: boolean },
  emit: {
    (e: 'update:show', value: boolean): void;
    (e: 'created', name: string): void;
  },
) {
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

  return {
    storage,
    account,
    options,
    result,
    step,
    busy,
    testing,
    error,
    saveAnyway,
    confirmed,
    gate,
    canSave,
    failing,
    runTest,
    next,
    save,
  };
}
