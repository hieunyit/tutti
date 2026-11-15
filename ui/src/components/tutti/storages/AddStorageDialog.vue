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
  import { useI18n } from 'vue-i18n';
  import StorageAccountForm from './StorageAccountForm.vue';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import StorageTestSteps from './StorageTestSteps.vue';
  import i18nStorages from './i18nStorages';
  import { useAddStorage } from './useAddStorage';
  import Callout from '@/components/ui/Callout.vue';

  const props = defineProps<{ show: boolean }>();
  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'created', name: string): void;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const {
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
  } = useAddStorage(props, emit);
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
