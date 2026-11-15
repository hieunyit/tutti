/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import i18nStorages from './i18nStorages';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { useAuthStore } from '@/stores/authStore';
import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
import type {
  RuntimeStorage,
  StorageTestResult,
} from '@/utils/types/runtimeStorage';

export type GroupKey = 'endpoint' | 'timing' | 'limits';

// each group of fields has its own form and Save button (spec 4.3)
export const GROUPS: { key: GroupKey; fields: (keyof RuntimeStorage)[] }[] = [
  {
    key: 'endpoint',
    fields: ['isSecure', 'address', 'provider', 'defaultRegion'],
  },
  { key: 'timing', fields: ['httpTimeout', 'healthCheckInterval'] },
  { key: 'limits', fields: ['rateLimit'] },
];

/** State and actions of the storage settings tab (forms, test, save, delete). */
export function useStorageSettings(
  props: { storage: RuntimeStorage },
  emit: { (e: 'saved'): void; (e: 'deleted'): void },
) {
  const { t } = useI18n({ messages: i18nStorages });
  const auth = useAuthStore();
  const draft = ref(RuntimeStorageHelper.empty());
  const original = ref(RuntimeStorageHelper.empty());
  const busy = ref(false);
  const error = ref('');
  const saved = ref(false);
  const confirmed = ref(false);
  const result = ref<StorageTestResult | null>(null);
  const testing = ref(false);
  const saveAnyway = ref(false);
  const forceConfirmed = ref(false);
  /** the backend refused the save because of failing steps: offer "save anyway" */
  const refused = ref(false);
  const rotating = ref<'rotate' | 'set' | null>(null);
  const deleteSection = ref<{ close: () => void } | null>(null);
  const saving = ref<GroupKey | null>(null);
  const isConfig = computed(() => props.storage.source !== 'managed');
  const readonly = computed(() => isConfig.value || !auth.canWrite);
  const account = computed(() => props.storage.connectionAccount);
  const hasAccount = computed(() => account.value?.configured === true);
  const shownTest = computed(
    () => result.value ?? props.storage.lastTest ?? null,
  );
  const fingerprint = computed(() =>
    JSON.stringify(RuntimeStorageHelper.payload(draft.value)),
  );
  const inUse = computed(() => (props.storage.references?.length ?? 0) > 0);

  function groupDirty(key: GroupKey) {
    const fields = GROUPS.find((group) => group.key === key)?.fields ?? [];
    const now = RuntimeStorageHelper.payload(draft.value);
    const before = RuntimeStorageHelper.payload(original.value);

    return fields.some(
      (field) => JSON.stringify(now[field]) !== JSON.stringify(before[field]),
    );
  }

  const dirty = computed(() => GROUPS.some((group) => groupDirty(group.key)));

  function canSave(key: GroupKey) {
    return (
      !readonly.value &&
      groupDirty(key) &&
      (!inUse.value || confirmed.value) &&
      (!refused.value || (saveAnyway.value && forceConfirmed.value))
    );
  }
  const groupText = (key: GroupKey, hint = false) =>
    t(
      `group${key === 'endpoint' ? 'Endpoint' : key === 'timing' ? 'Timing' : 'Limits'}${hint ? 'Hint' : ''}`,
    );

  function discardGroup(key: GroupKey) {
    const fields = GROUPS.find((group) => group.key === key)?.fields ?? [];
    const fresh = RuntimeStorageHelper.copy(
      original.value,
    ) as unknown as Record<string, unknown>;
    const next = { ...draft.value } as Record<string, unknown>;

    fields.forEach((field) => {
      next[field] = fresh[field];
    });
    draft.value = next as unknown as RuntimeStorage;
    error.value = '';
  }

  function discard() {
    original.value = RuntimeStorageHelper.copy(props.storage);
    draft.value = RuntimeStorageHelper.copy(props.storage);
    error.value = '';
    saved.value = false;
    confirmed.value = false;
    refused.value = false;
    saveAnyway.value = false;
    forceConfirmed.value = false;
  }
  watch(() => props.storage.name, discard, { immediate: true });
  watch(
    () => props.storage,
    () => {
      if (!dirty.value && !busy.value) discard();
    },
  );
  watch(fingerprint, () => {
    saved.value = false;
    refused.value = false;
    saveAnyway.value = false;
    forceConfirmed.value = false;
  });
  watch(saveAnyway, (value) => {
    if (!value) forceConfirmed.value = false;
  });

  async function testNow() {
    if (testing.value || readonly.value || !hasAccount.value) return;

    error.value = '';
    testing.value = true;

    try {
      result.value = await RuntimeStorageService.test({
        name: props.storage.name,
      });
      emit('saved');
    } catch (e) {
      error.value = RuntimeStorageHelper.error(e, t('testError'));
    } finally {
      testing.value = false;
    }
  }

  async function save(key: GroupKey) {
    if (busy.value || !canSave(key)) return;

    const fields = GROUPS.find((group) => group.key === key)?.fields ?? [];
    const picked = { ...original.value } as Record<string, unknown>;
    const edited = draft.value as unknown as Record<string, unknown>;

    fields.forEach((field) => {
      picked[field] = edited[field];
    });
    busy.value = true;
    saving.value = key;
    error.value = '';

    try {
      const storage = await RuntimeStorageService.update({
        storage: RuntimeStorageHelper.payload(
          picked as unknown as RuntimeStorage,
        ),
        confirm: confirmed.value,
        force: refused.value && saveAnyway.value && forceConfirmed.value,
      });
      const fresh = RuntimeStorageHelper.copy(storage) as unknown as Record<
        string,
        unknown
      >;

      // unsaved edits of the other groups stay in the draft
      GROUPS.filter((group) => group.key !== key).forEach((group) =>
        group.fields.forEach((field) => {
          fresh[field] = edited[field];
        }),
      );
      original.value = RuntimeStorageHelper.copy(storage);
      draft.value = fresh as unknown as RuntimeStorage;
      result.value = null;
      saved.value = true;
      refused.value = false;
      emit('saved');
    } catch (e) {
      error.value = RuntimeStorageHelper.error(e, t('error'));
      // the update re-tests the connection: a refusal lists the failing steps
      refused.value = hasAccount.value;
    } finally {
      busy.value = false;
      saving.value = null;
    }
  }

  async function remove() {
    if (busy.value || !RuntimeStorageHelper.canDelete(props.storage)) return;

    busy.value = true;
    error.value = '';

    try {
      await RuntimeStorageService.remove(props.storage.name);
      deleteSection.value?.close();
      emit('deleted');
    } catch (e) {
      error.value = RuntimeStorageHelper.error(e, t('error'));
    } finally {
      busy.value = false;
    }
  }

  return {
    groupDirty,
    dirty,
    canSave,
    groupText,
    discardGroup,
    discard,
    testNow,
    save,
    remove,
    draft,
    original,
    busy,
    error,
    saved,
    confirmed,
    testing,
    saveAnyway,
    forceConfirmed,
    refused,
    rotating,
    result,
    deleteSection,
    saving,
    isConfig,
    readonly,
    shownTest,
    inUse,
  };
}
