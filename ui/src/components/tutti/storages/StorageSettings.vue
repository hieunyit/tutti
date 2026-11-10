<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NCheckbox, NForm, NInput, NModal } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import StorageRotateDialog from './StorageRotateDialog.vue';
  import StorageTestSteps from './StorageTestSteps.vue';
  import i18nStorages from './i18nStorages';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import Callout from '@/components/ui/Callout.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { useAuthStore } from '@/stores/authStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
  import type {
    RuntimeStorage,
    StorageTestResult,
  } from '@/utils/types/runtimeStorage';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ storage: RuntimeStorage }>();
  const emit = defineEmits<{ (e: 'saved'): void; (e: 'deleted'): void }>();
  const { t, locale } = useI18n({ messages: i18nStorages });
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
  const deleting = ref(false);
  const deleteName = ref('');
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
      (!inUse.value || confirmed.value) &&
      (!refused.value || (saveAnyway.value && forceConfirmed.value)),
  );
  const time = (value?: string) => formatDateTime(value, locale.value);

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

  async function save() {
    if (busy.value || !canSave.value) return;

    busy.value = true;
    error.value = '';

    try {
      const storage = await RuntimeStorageService.update({
        storage: RuntimeStorageHelper.payload(draft.value),
        confirm: confirmed.value,
        force: refused.value && saveAnyway.value && forceConfirmed.value,
      });

      original.value = RuntimeStorageHelper.copy(storage);
      draft.value = RuntimeStorageHelper.copy(storage);
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
    <SectionCard :title="t('connectionAccount')">
      <template v-if="hasAccount">
        <dl class="storage-settings__facts">
          <div>
            <dt>{{ t('accessKey') }}</dt>
            <dd>
              <code>{{ account?.accessKeyId }}</code>
            </dd>
          </div>
          <div>
            <dt>{{ t('accountSource') }}</dt>
            <dd>
              <StatusTag
                no-dot
                :label="
                  account?.source === 'config' ? t('config') : t('managed')
                "
              />
            </dd>
          </div>
          <div v-if="account?.rotatedAt">
            <dt>{{ t('rotatedAt') }}</dt>
            <dd>{{ time(account.rotatedAt) }}</dd>
          </div>
          <div v-if="account?.previousAccessKeyId">
            <dt>{{ t('previousKey') }}</dt>
            <dd>
              <code>{{ account.previousAccessKeyId }}</code>
              <small v-if="account.safeToRevokePreviousAfter">{{
                Date.parse(account.safeToRevokePreviousAfter) > Date.now()
                  ? t('safeToRevoke', {
                      time: time(account.safeToRevokePreviousAfter),
                    })
                  : t('revokeNow')
              }}</small>
            </dd>
          </div>
        </dl>
        <p
          v-if="account?.source === 'config'"
          class="storage-settings__sub"
        >
          {{ t('configAccountHint') }}
        </p>
      </template>
      <template v-else>
        <p>
          <StatusTag variant="warning">{{ t('legacy') }}</StatusTag>
        </p>
        <Callout
          v-if="storage.legacyCredentials !== false"
          tone="warning"
          >{{ t('legacyHint') }}</Callout
        >
      </template>
      <div
        v-if="!isConfig"
        class="storage-settings__actions"
      >
        <WriteGate
          ><NButton
            :disabled="busy"
            @click="rotating = hasAccount ? 'rotate' : 'set'"
            >{{ hasAccount ? t('rotate') : t('setAccount') }}</NButton
          ></WriteGate
        >
      </div>
      <RouterLink
        class="storage-settings__link"
        :to="{
          name: RouteName.TUTTI_DOCS,
          params: { slug: 'storage-accounts' },
        }"
        >{{ t('guideLink') }}</RouterLink
      >
    </SectionCard>
    <SectionCard :title="t('lastTest')">
      <p
        v-if="storage.lastForcedSaveBy"
        class="storage-settings__sub"
      >
        {{
          t('lastForced', {
            by: storage.lastForcedSaveBy,
            time: time(storage.lastForcedSaveAt),
          })
        }}
      </p>
      <StorageTestSteps :result="shownTest" />
      <div class="storage-settings__actions">
        <WriteGate
          ><NButton
            :loading="testing"
            :disabled="!hasAccount || busy"
            @click="testNow"
            >{{ t('testNow') }}</NButton
          ></WriteGate
        >
      </div>
    </SectionCard>
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
          <div class="storage-settings__actions">
            <NButton
              :disabled="busy || !dirty"
              @click="discard"
              >{{ t('discard') }}</NButton
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
    ><StorageRotateDialog
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
