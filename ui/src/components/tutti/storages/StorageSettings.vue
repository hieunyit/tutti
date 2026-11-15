<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NCheckbox, NForm } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import StorageAccountPanel from './StorageAccountPanel.vue';
  import StorageConnectionForm from './StorageConnectionForm.vue';
  import StorageDeleteSection from './StorageDeleteSection.vue';
  import StorageRotateDialog from './StorageRotateDialog.vue';
  import i18nStorages from './i18nStorages';
  import { GROUPS, useStorageSettings } from './useStorageSettings';
  import { Callout, FormSection, SectionCard } from '@/components/kit';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

  const props = defineProps<{ storage: RuntimeStorage }>();
  const emit = defineEmits<{ (e: 'saved'): void; (e: 'deleted'): void }>();
  const { t } = useI18n({ messages: i18nStorages });
  const {
    groupDirty,
    dirty,
    canSave,
    groupText,
    discardGroup,
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
  } = useStorageSettings(props, emit);
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

    &__unsaved {
      margin-right: auto;
      align-self: center;
      color: var(--tt-warning);
      font-size: var(--tt-text-sm);
    }

    &__success {
      color: var(--tt-success);
    }

    li {
      overflow-wrap: anywhere;
    }
  }
</style>
