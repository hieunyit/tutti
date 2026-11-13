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
  import {
    NButton,
    NForm,
    NFormItem,
    NInputNumber,
    NModal,
    NRadio,
    NSpace,
    NSelect,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtection';
  import Callout from '@/components/ui/Callout.vue';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionFormError,
    ProtectionGroup,
    ProtectionGroupForm,
  } from '@/utils/types/protection';

  const props = defineProps<{
    show: boolean;
    /** group being edited */
    group: ProtectionGroup;
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved', id: string): void;
  }>();

  const { t } = useI18n({ messages: i18nProtection });
  const store = useTuttiProtectionStore();
  const { proxyIds } = storeToRefs(store);

  const form = ref<ProtectionGroupForm>(ProtectionGroupHelper.emptyForm());
  const showErrors = ref(false);
  const isSaving = ref(false);
  const backendError = ref<string | null>(null);

  watch(
    () => props.show,
    (show) => {
      if (!show) return;

      store.loadOptions();
      form.value = ProtectionGroupHelper.groupToForm(props.group);
      showErrors.value = false;
      backendError.value = null;
      isSaving.value = false;
    },
    { immediate: true },
  );

  const errors = computed<ProtectionFormError[]>(() =>
    ProtectionGroupHelper.validate(form.value, true),
  );

  function errorText(...codes: ProtectionFormError[]): string | undefined {
    if (!showErrors.value) return undefined;

    const found = errors.value.find((code) => codes.includes(code));

    return found ? t(found) : undefined;
  }

  const proxyOptions = computed(() =>
    proxyIds.value.map((id) => ({ label: id, value: id })),
  );

  async function submit() {
    showErrors.value = true;
    backendError.value = null;

    if (errors.value.length > 0) return;

    isSaving.value = true;

    const err = await store.updateGroup(props.group, form.value);

    isSaving.value = false;

    if (err === null) {
      emit('saved', form.value.id.trim());
      emit('update:show', false);

      return;
    }

    backendError.value = t('saveFailed', { message: err });
  }
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('dialogEdit', { id: group.id })"
    :closable="!isSaving"
    :mask-closable="!isSaving"
    :style="{ width: 'min(640px, calc(100vw - 32px))' }"
    @update:show="emit('update:show', $event)"
  >
    <NForm
      class="protection-dialog__form"
      label-placement="top"
      @submit.prevent="submit"
    >
      <Callout tone="info">
        {{ t('immutableNote') }}
      </Callout>

      <div class="protection-dialog__grid">
        <NFormItem
          :label="t('fieldCopies')"
          :validation-status="errorText('COPIES_RANGE') ? 'error' : undefined"
          :feedback="errorText('COPIES_RANGE') ?? t('fieldCopiesHelp')"
        >
          <NInputNumber
            v-model:value="form.copies"
            :min="2"
            :max="Math.max(form.members.length, 2)"
            :precision="0"
            :disabled="isSaving"
          />
        </NFormItem>

        <NFormItem
          :label="t('fieldMaxLag')"
          :validation-status="errorText('LAG_NEGATIVE') ? 'error' : undefined"
          :feedback="errorText('LAG_NEGATIVE') ?? t('fieldMaxLagHelp')"
        >
          <NInputNumber
            v-model:value="form.maxEventLag"
            :min="0"
            :precision="0"
            :disabled="isSaving"
            placeholder="0"
            clearable
          />
        </NFormItem>
      </div>

      <NFormItem :label="t('fieldMode')">
        <NSpace
          vertical
          size="small"
        >
          <NRadio
            :checked="form.mode === 'manual'"
            value="manual"
            :disabled="isSaving"
            @update:checked="form.mode = 'manual'"
          >
            {{ t('fieldModeManual') }}
          </NRadio>
          <NRadio
            :checked="form.mode === 'auto'"
            value="auto"
            :disabled="isSaving"
            @update:checked="form.mode = 'auto'"
          >
            {{ t('fieldModeAuto') }}
          </NRadio>
        </NSpace>
      </NFormItem>

      <NFormItem
        v-if="form.mode === 'auto'"
        :label="t('fieldElectorate')"
        :validation-status="errorText('ELECTORATE_MIN') ? 'error' : undefined"
        :feedback="errorText('ELECTORATE_MIN') ?? t('fieldElectorateHelp')"
      >
        <NSelect
          v-model:value="form.electorate"
          :options="proxyOptions"
          :disabled="isSaving"
          multiple
          filterable
          tag
        />
      </NFormItem>

      <div class="protection-dialog__grid">
        <NFormItem
          :label="t('fieldAfter')"
          :validation-status="errorText('AFTER_NEGATIVE') ? 'error' : undefined"
          :feedback="errorText('AFTER_NEGATIVE') ?? t('fieldAfterHelp')"
        >
          <NInputNumber
            v-model:value="form.afterSeconds"
            :min="0"
            :precision="0"
            :disabled="isSaving"
            placeholder="30"
            clearable
          />
        </NFormItem>

        <NFormItem
          :label="t('fieldSwitches')"
          :validation-status="
            errorText('SWITCHES_POSITIVE') ? 'error' : undefined
          "
          :feedback="errorText('SWITCHES_POSITIVE') ?? t('fieldSwitchesHelp')"
        >
          <NInputNumber
            v-model:value="form.maxSwitchesPerHour"
            :min="1"
            :precision="0"
            :disabled="isSaving"
            placeholder="3"
            clearable
          />
        </NFormItem>
      </div>

      <Callout
        v-if="backendError"
        tone="danger"
      >
        {{ backendError }}
      </Callout>
    </NForm>

    <template #footer>
      <div class="protection-dialog__footer">
        <NButton
          :disabled="isSaving"
          @click="emit('update:show', false)"
        >
          {{ t('cancel') }}
        </NButton>
        <NButton
          type="primary"
          :loading="isSaving"
          @click="submit"
        >
          {{ t('save') }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style lang="scss" scoped>
  .protection-dialog {
    &__form {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    &__stack {
      display: flex;
      flex-direction: column;
      gap: 8px;
      width: 100%;
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    &__tag-item {
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }

    &__inline {
      display: flex;
      gap: 8px;
      width: 100%;
    }

    &__empty {
      margin: 0;
      font-size: 13px;
      color: var(--tt-text-3);
    }

    &__member {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    &__member-name {
      flex: 1;
      font-weight: 500;
      min-width: 0;
      overflow-wrap: anywhere;
    }

    &__grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;

      @media (max-width: 560px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }

    &__adopt {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    &__adopt-help {
      margin: 0;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__footer {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }
  }
</style>
