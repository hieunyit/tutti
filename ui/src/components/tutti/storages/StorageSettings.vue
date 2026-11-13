<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NCheckbox, NForm } from 'naive-ui';
  import { computed, ref, useTemplateRef, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageAccountPanel from './StorageAccountPanel.vue';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import StorageDeleteSection from './StorageDeleteSection.vue';
  import StorageRotateDialog from './StorageRotateDialog.vue';
  import i18nStorages from './i18nStorages';
  import { Callout, FormSection, SectionCard } from '@/components/kit';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { useAuthStore } from '@/stores/authStore';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import type {
    RuntimeStorage,
    StorageTestResult,
  } from '@/utils/types/runtimeStorage';

  type GroupKey = 'endpoint' | 'timing' | 'limits';

  // each group of fields has its own form and Save button (spec 4.3)
  const GROUPS: { key: GroupKey; fields: (keyof RuntimeStorage)[] }[] = [
    {
      key: 'endpoint',
      fields: ['isSecure', 'address', 'provider', 'defaultRegion'],
    },
    { key: 'timing', fields: ['httpTimeout', 'healthCheckInterval'] },
    { key: 'limits', fields: ['rateLimit'] },
  ];
  const props = defineProps<{ storage: RuntimeStorage }>();
  const emit = defineEmits<{ (e: 'saved'): void; (e: 'deleted'): void }>();
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
  const deleteSection = useTemplateRef('deleteSection');
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
</script>

<template>
  <div class="storage-settings">
    <Callout v-if="isConfig">{{ t('configHint') }}</Callout>
    <Callout
      v-if="inUse"
      tone="warning"
      :title="t('inUse')"
      ><ul>
        <li
          v-for="reference in storage.references"
          :key="reference"
        >
          {{ reference }}
        </li>
      </ul></Callout
    >
    <Callout
      v-if="error"
      tone="danger"
      >{{ error }}</Callout
    >
    <p
      v-if="saved"
      class="storage-settings__success"
      role="status"
    >
      {{ t('saved') }}
    </p>
    <StorageAccountPanel
      :storage="storage"
      :is-config="isConfig"
      :busy="busy"
      :testing="testing"
      :shown-test="shownTest"
      @rotate="rotating = $event"
      @test="testNow"
    />
    <FormSection
      v-for="group in GROUPS"
      :key="group.key"
      :title="groupText(group.key)"
      :description="groupText(group.key, true)"
    >
      <NForm
        label-placement="top"
        @submit.prevent="save(group.key)"
      >
        <StorageConnectionForm
          v-model="draft"
          :original="original"
          :readonly="readonly || busy"
          :group="group.key"
        />
        <div
          v-if="!readonly"
          class="storage-settings__actions"
        >
          <span
            v-if="groupDirty(group.key)"
            class="storage-settings__unsaved"
            role="status"
            >{{ t('unsavedSection') }}</span
          >
          <NButton
            :disabled="busy || !groupDirty(group.key)"
            @click="discardGroup(group.key)"
            >{{ t('discard') }}</NButton
          ><NButton
            type="primary"
            attr-type="submit"
            :disabled="!canSave(group.key)"
            :loading="saving === group.key"
            >{{ t('save') }}</NButton
          >
        </div>
      </NForm>
    </FormSection>
    <SectionCard
      v-if="!readonly && (inUse || refused || dirty)"
      :title="t('saveOptions')"
    >
      <NCheckbox
        v-if="inUse"
        v-model:checked="confirmed"
        :disabled="busy"
        >{{ t('confirmUse') }}</NCheckbox
      >
      <template v-if="refused">
        <NCheckbox
          v-model:checked="saveAnyway"
          :disabled="busy"
          >{{ t('saveAnyway') }} · {{ t('saveAnywayHint') }}</NCheckbox
        ><NCheckbox
          v-if="saveAnyway"
          v-model:checked="forceConfirmed"
          :disabled="busy"
          >{{ t('saveAnywayConfirm') }}</NCheckbox
        >
      </template>
      <p
        v-else
        class="storage-settings__sub"
      >
        {{ t('testRequired') }}
      </p>
    </SectionCard>
    <StorageDeleteSection
      v-if="!readonly"
      ref="deleteSection"
      :storage="storage"
      :busy="busy"
      :error="error"
      @confirm="remove"
    />
    <StorageRotateDialog
      :show="rotating !== null"
      :storage="storage"
      :mode="rotating ?? 'rotate'"
      @update:show="!$event && (rotating = null)"
      @done="
        result = null;
        emit('saved');
      "
    />
  </div>
</template>

<style scoped lang="scss">
  .storage-settings {
    display: flex;
    flex-direction: column;
    gap: 16px;

    &__actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 16px;
    }

    &__sub {
      color: var(--tt-text-3);
      font-size: 13px;
    }

    &__facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 12px 16px;
      margin: 0;

      dt {
        color: var(--tt-text-3);
        font-size: 12px;
      }

      dd {
        margin: 2px 0 0;
        overflow-wrap: anywhere;
      }

      small {
        display: block;
        color: var(--tt-text-3);
      }
    }

    &__link {
      display: inline-block;
      margin-top: 12px;
      color: var(--tt-accent);
    }

    &__unsaved {
      margin-right: auto;
      align-self: center;
      color: var(--tt-warning);
      font-size: var(--tt-text-sm);
    }

    &__success {
      color: var(--tt-success);
    }

    &__error {
      color: var(--tt-danger);
      overflow-wrap: anywhere;
    }

    li {
      overflow-wrap: anywhere;
    }
  }
</style>
