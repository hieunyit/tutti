<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  import { NModal } from 'naive-ui';
  import { computed, nextTick, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtection';
  import WizardMembersStep from './WizardMembersStep.vue';
  import WizardOptionsStep from './WizardOptionsStep.vue';
  import WizardScopeStep from './WizardScopeStep.vue';
  import WizardSummary from './WizardSummary.vue';
  import {
    ErrorBlock,
    WizardShell,
    clearWizardDraft,
    useWizardDraft,
  } from '@/components/kit';
  import {
    OVERLAP_ERROR,
    useTuttiProtectionStore,
  } from '@/stores/tuttiProtectionStore';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionFormError,
    ProtectionGroupForm,
  } from '@/utils/types/protection';

  const DRAFT_KEY = 'protection-group-create';

  const props = defineProps<{ show: boolean }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved', id: string): void;
  }>();

  const { t } = useI18n({ messages: i18nProtection });
  const store = useTuttiProtectionStore();

  const form = useWizardDraft<ProtectionGroupForm>(DRAFT_KEY, () =>
    ProtectionGroupHelper.emptyForm(),
  );
  const scopeStep = ref<InstanceType<typeof WizardScopeStep> | null>(null);
  const isSaving = ref(false);
  const backendError = ref<string | null>(null);
  const errorAnchor = ref<HTMLElement | null>(null);
  const touched = ref(false);

  const errors = computed<ProtectionFormError[]>(() =>
    ProtectionGroupHelper.validate(form.value, false),
  );
  const SCOPE_ERRORS: ProtectionFormError[] = [
    'ID_REQUIRED',
    'ID_INVALID',
    'BUCKET_INVALID',
  ];
  const steps = computed(() => [
    {
      key: 'scope',
      label: t('wizStepScope'),
      valid: !errors.value.some((code) => SCOPE_ERRORS.includes(code)),
    },
    {
      key: 'members',
      label: t('wizStepMembers'),
      valid: errors.value.length === 0,
    },
  ]);

  watch(
    () => form.value.members.length,
    (count) => {
      form.value.copies = ProtectionGroupHelper.clampCopies(
        form.value.copies,
        count,
      );
    },
  );
  watch(
    () => form.value.id,
    () => {
      backendError.value = null;
    },
  );
  watch(
    () => props.show,
    (open) => {
      if (open) store.loadOptions();
    },
    { immediate: true },
  );

  function errorText(...codes: ProtectionFormError[]): string | undefined {
    if (!touched.value) return undefined;

    const found = errors.value.find((code) => codes.includes(code));

    return found ? t(found) : undefined;
  }

  function close() {
    emit('update:show', false);
  }

  async function finish() {
    scopeStep.value?.addBucket();

    if (errors.value.length > 0) {
      touched.value = true;

      return;
    }

    isSaving.value = true;
    backendError.value = null;

    const err = await store.createGroup(form.value);

    isSaving.value = false;

    if (err !== null) {
      backendError.value =
        err === OVERLAP_ERROR
          ? t('overlapError')
          : t('saveFailed', { message: err });

      // the block sits below the long form: bring it into view, the submit
      // button in the footer would otherwise look dead
      await nextTick();
      errorAnchor.value?.scrollIntoView?.({
        block: 'nearest',
        behavior: 'smooth',
      });

      return;
    }

    const id = form.value.id.trim();

    clearWizardDraft(DRAFT_KEY);
    emit('saved', id);
    close();
  }
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('wizTitle')"
    :closable="!isSaving"
    :mask-closable="false"
    :style="{ width: 'min(760px, calc(100vw - 32px))' }"
    @update:show="!$event && !isSaving && close()"
  >
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('wizFinish')"
      :busy="isSaving"
      @finish="finish"
      @step="touched = false"
    >
      <template #scope>
        <WizardScopeStep
          ref="scopeStep"
          v-model="form"
          :error-text="errorText"
          @blur="touched = true"
        />
      </template>

      <template #members>
        <WizardMembersStep
          v-model="form"
          :error-text="errorText"
        />
        <WizardOptionsStep
          v-model="form"
          :error-text="errorText"
        />
        <div
          v-if="backendError"
          ref="errorAnchor"
        >
          <ErrorBlock
            :title="t('wizSaveFailed')"
            :description="backendError"
            @retry="finish"
          />
        </div>
      </template>

      <template #summary>
        <WizardSummary :form="form" />
      </template>
    </WizardShell>
  </NModal>
</template>
