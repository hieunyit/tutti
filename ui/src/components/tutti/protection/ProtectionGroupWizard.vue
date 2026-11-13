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
  import { NFormItem, NInput, NModal, NTag } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtectionUi';
  import WizardMembersStep from './WizardMembersStep.vue';
  import WizardOptionsStep from './WizardOptionsStep.vue';
  import {
    ErrorBlock,
    FormSection,
    WizardShell,
    clearWizardDraft,
    useWizardDraft,
  } from '@/components/kit';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
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
  const bucketInput = ref('');
  const isSaving = ref(false);
  const backendError = ref<string | null>(null);
  const touched = ref(false);

  const errors = computed<ProtectionFormError[]>(() =>
    ProtectionGroupHelper.validate(form.value, false),
  );
  const SCOPE_ERRORS: ProtectionFormError[] = [
    'ID_REQUIRED',
    'ID_INVALID',
    'BUCKET_INVALID',
  ];
  const scopeValid = computed(
    () => !errors.value.some((code) => SCOPE_ERRORS.includes(code)),
  );
  const membersValid = computed(
    () => !errors.value.some((code) => !SCOPE_ERRORS.includes(code)),
  );
  const steps = computed(() => [
    { key: 'scope', label: t('wizStepScope'), valid: scopeValid.value },
    { key: 'members', label: t('wizStepMembers'), valid: membersValid.value },
    { key: 'confirm', label: t('wizStepConfirm'), valid: true },
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

  function errorText(...codes: ProtectionFormError[]): string | undefined {
    if (!touched.value) return undefined;

    const found = errors.value.find((code) => codes.includes(code));

    return found ? t(found) : undefined;
  }

  watch(
    () => props.show,
    (open) => {
      if (open) store.loadOptions();
    },
    { immediate: true },
  );

  function addBucket() {
    const value = bucketInput.value.trim();

    if (!value) return;

    form.value.buckets = [...form.value.buckets, value];
    bucketInput.value = '';
  }

  function close() {
    emit('update:show', false);
  }

  function onStep() {
    touched.value = false;
  }

  async function finish() {
    if (bucketInput.value.trim() !== '') addBucket();

    if (errors.value.length > 0) {
      touched.value = true;

      return;
    }

    isSaving.value = true;
    backendError.value = null;

    const err = await store.createGroup(form.value);

    isSaving.value = false;

    if (err !== null) {
      backendError.value = t('saveFailed', { message: err });

      return;
    }

    const id = form.value.id.trim();

    clearWizardDraft(DRAFT_KEY);
    bucketInput.value = '';
    emit('saved', id);
    close();
  }

  const scopeSummary = computed(() =>
    form.value.buckets.length
      ? form.value.buckets.join(', ')
      : t('wizAllBuckets'),
  );
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('wizTitle')"
    :closable="!isSaving"
    :mask-closable="false"
    :style="{ width: 'min(680px, calc(100vw - 32px))' }"
    @update:show="!$event && !isSaving && close()"
  >
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('wizFinish')"
      :busy="isSaving"
      @finish="finish"
      @step="onStep"
    >
      <template #scope>
        <FormSection
          :title="t('wizStepScope')"
          :description="t('wizScopeDesc')"
        >
          <NFormItem
            :label="t('fieldId')"
            :validation-status="
              errorText('ID_REQUIRED', 'ID_INVALID') ? 'error' : undefined
            "
            :feedback="
              errorText('ID_REQUIRED', 'ID_INVALID') ?? t('fieldIdHelp')
            "
          >
            <NInput
              v-model:value="form.id"
              @blur="touched = true"
            />
          </NFormItem>
          <NFormItem
            :label="t('fieldBuckets')"
            :validation-status="
              errorText('BUCKET_INVALID') ? 'error' : undefined
            "
            :feedback="errorText('BUCKET_INVALID') ?? t('fieldBucketsHelp')"
          >
            <div class="pg-wizard__stack">
              <div
                v-if="form.buckets.length"
                class="pg-wizard__row"
              >
                <NTag
                  v-for="(bucket, index) in form.buckets"
                  :key="`${bucket}-${index}`"
                  round
                  closable
                  @close="
                    form.buckets = form.buckets.filter((_, i) => i !== index)
                  "
                >
                  {{ bucket }}
                </NTag>
              </div>
              <NInput
                v-model:value="bucketInput"
                :placeholder="t('fieldBucketsPlaceholder')"
                @keydown.enter.prevent="addBucket"
                @blur="addBucket"
              />
            </div>
          </NFormItem>
        </FormSection>
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
      </template>

      <template #confirm>
        <FormSection
          :title="t('wizStepConfirm')"
          :description="t('wizConfirmDesc')"
        >
          <dl class="pg-wizard__summary">
            <div>
              <dt>{{ t('fieldId') }}</dt>
              <dd class="tt-mono">{{ form.id.trim() }}</dd>
            </div>
            <div>
              <dt>{{ t('colScope') }}</dt>
              <dd>{{ scopeSummary }}</dd>
            </div>
            <div>
              <dt>{{ t('wizPrimary') }}</dt>
              <dd class="tt-mono">{{ form.members[0] }}</dd>
            </div>
            <div>
              <dt>{{ t('fieldMembers') }}</dt>
              <dd class="tt-mono">{{ form.members.join(' → ') }}</dd>
            </div>
            <div>
              <dt>{{ t('fieldCopies') }}</dt>
              <dd>{{ form.copies }}</dd>
            </div>
            <div>
              <dt>{{ t('fieldMode') }}</dt>
              <dd>
                {{ form.mode === 'auto' ? t('modeAuto') : t('modeManual') }}
              </dd>
            </div>
          </dl>
          <ErrorBlock
            v-if="backendError"
            :title="t('wizSaveFailed')"
            :description="backendError"
            @retry="finish"
          />
        </FormSection>
      </template>
    </WizardShell>
  </NModal>
</template>

<style lang="scss" scoped>
  .pg-wizard {
    &__stack {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      width: 100%;
    }

    &__row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-2);
      width: 100%;
    }

    &__member {
      flex: 1;
      min-width: 0;
      overflow-wrap: anywhere;
    }

    &__hint {
      margin: 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }

    &__summary {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: var(--tt-space-3) var(--tt-space-5);
      margin: 0;

      dt {
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
      }

      dd {
        margin: var(--tt-space-1) 0 0;
        font-weight: var(--tt-weight-medium);
        overflow-wrap: anywhere;
      }
    }
  }
</style>
