<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NCheckbox, NForm, NInput, NModal } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import i18nStorages from './i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { useAuthStore } from '@/stores/authStore';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import type {
    RuntimeStorage,
    RuntimeStorageTestResult,
  } from '@/utils/types/runtimeStorage';

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
  const tested = ref('');
  const result = ref<RuntimeStorageTestResult | null>(null);
  const deleting = ref(false);
  const deleteName = ref('');
  const readonly = computed(
    () => props.storage.source !== 'managed' || !auth.canWrite,
  );
  const fingerprint = computed(() =>
    JSON.stringify(RuntimeStorageHelper.payload(draft.value)),
  );
  const dirty = computed(
    () =>
      fingerprint.value !==
      JSON.stringify(RuntimeStorageHelper.payload(original.value)),
  );
  const inUse = computed(() => (props.storage.references?.length ?? 0) > 0);
  const canSave = computed(
    () =>
      !readonly.value &&
      dirty.value &&
      tested.value === fingerprint.value &&
      (!inUse.value || confirmed.value),
  );

  function discard() {
    original.value = RuntimeStorageHelper.copy(props.storage);
    draft.value = RuntimeStorageHelper.copy(props.storage);
    error.value = '';
    saved.value = false;
    confirmed.value = false;
    tested.value = '';
    result.value = null;
  }
  watch(() => props.storage.name, discard, { immediate: true });
  watch(
    () => props.storage,
    () => {
      if (!dirty.value && !busy.value) discard();
    },
  );
  watch(fingerprint, () => {
    tested.value = '';
    result.value = null;
    saved.value = false;
  });

  async function test() {
    if (busy.value || readonly.value) return;

    error.value = '';

    if (!RuntimeStorageHelper.validConnection(draft.value)) {
      error.value = t('invalid');

      return;
    }

    busy.value = true;
    const version = fingerprint.value;

    try {
      const response = await RuntimeStorageService.test(
        RuntimeStorageHelper.payload(draft.value),
      );

      if (version !== fingerprint.value) return;

      result.value = response;

      if (response.ok) tested.value = version;
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
      const storage = await RuntimeStorageService.update({
        storage: RuntimeStorageHelper.payload(draft.value),
        confirm: confirmed.value,
      });

      original.value = RuntimeStorageHelper.copy(storage);
      draft.value = RuntimeStorageHelper.copy(storage);
      saved.value = true;
      emit('saved');
    } catch (e) {
      error.value = RuntimeStorageHelper.error(e, t('error'));
    } finally {
      busy.value = false;
    }
  }

  async function remove() {
    if (
      busy.value ||
      deleteName.value !== props.storage.name ||
      !RuntimeStorageHelper.canDelete(props.storage)
    )
      return;

    busy.value = true;
    error.value = '';

    try {
      await RuntimeStorageService.remove(props.storage.name);
      deleting.value = false;
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
    <Callout v-if="storage.source !== 'managed'">{{ t('configHint') }}</Callout>
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
    <SectionCard :title="t('connection')">
      <NForm
        label-placement="top"
        @submit.prevent="save"
        ><StorageConnectionForm
          v-model="draft"
          :original="original"
          :readonly="readonly || busy"
        />
        <template v-if="!readonly"
          ><NCheckbox
            v-if="inUse"
            v-model:checked="confirmed"
            :disabled="busy"
            >{{ t('confirmUse') }}</NCheckbox
          >
          <p
            v-if="result"
            role="status"
            :class="
              result.ok
                ? 'storage-settings__success'
                : 'storage-settings__error'
            "
          >
            {{
              result.ok
                ? t('testOk', { ms: result.latencyMs ?? 0 })
                : result.error || t('testFailed')
            }}
          </p>
          <p
            v-else
            class="storage-settings__sub"
          >
            {{ t('testRequired') }}
          </p>
          <div class="storage-settings__actions">
            <NButton
              :disabled="busy || !dirty"
              @click="discard"
              >{{ t('discard') }}</NButton
            ><NButton
              :loading="busy"
              @click="test"
              >{{ t('test') }}</NButton
            ><NButton
              type="primary"
              attr-type="submit"
              :disabled="!canSave"
              :loading="busy"
              >{{ t('save') }}</NButton
            >
          </div>
        </template>
      </NForm>
    </SectionCard>
    <SectionCard
      v-if="!readonly"
      :title="t('deleteTitle')"
      ><p>{{ t('deleteHint') }}</p>
      <p
        v-if="!RuntimeStorageHelper.canDelete(storage)"
        class="storage-settings__sub"
      >
        {{ t('deleteBlocked') }}
      </p>
      <NButton
        type="error"
        secondary
        :disabled="busy || !RuntimeStorageHelper.canDelete(storage)"
        @click="
          deleting = true;
          deleteName = '';
        "
        >{{ t('delete') }}</NButton
      ></SectionCard
    >
    <NModal
      v-model:show="deleting"
      preset="card"
      :title="t('deleteTitle')"
      :closable="!busy"
      :mask-closable="!busy"
      :style="{ width: 'min(480px, calc(100vw - 32px))' }"
      ><Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <p>{{ t('deleteConfirm', { name: storage.name }) }}</p>
      <NInput
        v-model:value="deleteName"
        :aria-label="t('name')"
        :disabled="busy"
      /><template #footer
        ><div class="storage-settings__actions">
          <NButton
            :disabled="busy"
            @click="deleting = false"
            >{{ t('cancel') }}</NButton
          ><NButton
            type="error"
            :disabled="deleteName !== storage.name"
            :loading="busy"
            @click="remove"
            >{{ t('delete') }}</NButton
          >
        </div></template
      ></NModal
    >
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
