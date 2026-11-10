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
    NCheckbox,
    NForm,
    NFormItem,
    NInput,
    NInputNumber,
    NModal,
    NRadio,
    NSpace,
    NSelect,
    NTag,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtection';
  import Callout from '@/components/ui/Callout.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionFormError,
    ProtectionGroup,
    ProtectionGroupForm,
  } from '@/utils/types/protection';

  const props = defineProps<{
    show: boolean;
    /** group being edited; null/undefined means create mode */
    group?: ProtectionGroup | null;
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved', id: string): void;
  }>();

  const { t } = useI18n({ messages: i18nProtection });
  const store = useTuttiProtectionStore();
  const { userNames, storageNames, proxyIds } = storeToRefs(store);

  const form = ref<ProtectionGroupForm>(ProtectionGroupHelper.emptyForm());
  const bucketInput = ref('');
  const selectedStorage = ref<string | null>(null);
  const showErrors = ref(false);
  const isSaving = ref(false);
  const backendError = ref<string | null>(null);

  const isEdit = computed(() => !!props.group);

  watch(
    () => props.show,
    (show) => {
      if (!show) return;

      store.loadOptions();
      form.value = props.group
        ? ProtectionGroupHelper.groupToForm(props.group)
        : ProtectionGroupHelper.emptyForm();
      bucketInput.value = '';
      selectedStorage.value = null;
      showErrors.value = false;
      backendError.value = null;
      isSaving.value = false;
    },
    { immediate: true },
  );

  const errors = computed<ProtectionFormError[]>(() =>
    ProtectionGroupHelper.validate(form.value, isEdit.value),
  );

  function errorText(...codes: ProtectionFormError[]): string | undefined {
    if (!showErrors.value) return undefined;

    const found = errors.value.find((code) => codes.includes(code));

    return found ? t(found) : undefined;
  }

  const userOptions = computed(() =>
    userNames.value.map((name) => ({ label: name, value: name })),
  );

  const availableStorageOptions = computed(() =>
    storageNames.value
      .filter((name) => !form.value.members.includes(name))
      .map((name) => ({ label: name, value: name })),
  );

  const proxyOptions = computed(() =>
    proxyIds.value.map((id) => ({ label: id, value: id })),
  );

  function addBucket() {
    const trimmed = bucketInput.value.trim();

    if (!trimmed) return;

    form.value.buckets = [...form.value.buckets, trimmed];
    bucketInput.value = '';
  }

  function removeBucket(index: number) {
    form.value.buckets = form.value.buckets.filter((_, i) => i !== index);
  }

  function addMemberStorage() {
    if (!selectedStorage.value) return;

    form.value.members = ProtectionGroupHelper.addMember(
      form.value.members,
      selectedStorage.value,
    );
    form.value.copies = ProtectionGroupHelper.clampCopies(
      form.value.copies,
      form.value.members.length,
    );
    selectedStorage.value = null;
  }

  function moveMember(index: number, dir: -1 | 1) {
    form.value.members = ProtectionGroupHelper.moveMember(
      form.value.members,
      index,
      dir,
    );
  }

  function removeMemberStorage(storage: string) {
    form.value.members = ProtectionGroupHelper.removeMember(
      form.value.members,
      storage,
    );
    form.value.copies = ProtectionGroupHelper.clampCopies(
      form.value.copies,
      form.value.members.length,
    );
  }

  async function submit() {
    if (!isEdit.value && bucketInput.value.trim() !== '') {
      addBucket();
    }

    showErrors.value = true;
    backendError.value = null;

    if (errors.value.length > 0) return;

    isSaving.value = true;

    const err =
      isEdit.value && props.group
        ? await store.updateGroup(props.group, form.value)
        : await store.createGroup(form.value);

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
    :title="isEdit ? t('dialogEdit', { id: group?.id ?? '' }) : t('dialogCreate')"
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
      <Callout
        v-if="isEdit"
        tone="info"
      >
        {{ t('immutableNote') }}
      </Callout>

      <template v-else>
        <NFormItem
          :label="t('fieldId')"
          :validation-status="
            errorText('ID_REQUIRED', 'ID_INVALID') ? 'error' : undefined
          "
          :feedback="errorText('ID_REQUIRED', 'ID_INVALID') ?? t('fieldIdHelp')"
        >
          <NInput
            v-model:value="form.id"
            :disabled="isSaving"
          />
        </NFormItem>

        <NFormItem
          :label="t('fieldUser')"
          :validation-status="errorText('USER_REQUIRED') ? 'error' : undefined"
          :feedback="errorText('USER_REQUIRED')"
        >
          <NSelect
            v-model:value="form.user"
            :options="userOptions"
            :placeholder="t('fieldUserPlaceholder')"
            :disabled="isSaving"
            filterable
            tag
          />
        </NFormItem>

        <NFormItem
          :label="t('fieldBuckets')"
          :validation-status="errorText('BUCKET_INVALID') ? 'error' : undefined"
          :feedback="errorText('BUCKET_INVALID') ?? t('fieldBucketsHelp')"
        >
          <div class="protection-dialog__stack">
            <div
              v-if="form.buckets.length > 0"
              class="protection-dialog__tags"
            >
              <span
                v-for="(bucket, index) in form.buckets"
                :key="`${bucket}-${index}`"
                class="protection-dialog__tag-item"
              >
                <NTag round>{{ bucket }}</NTag>
                <NButton
                  size="tiny"
                  :disabled="isSaving"
                  :aria-label="t('remove')"
                  @click="removeBucket(index)"
                >
                  ✕
                </NButton>
              </span>
            </div>
            <div class="protection-dialog__inline">
              <NInput
                v-model:value="bucketInput"
                :placeholder="t('fieldBucketsPlaceholder')"
                :disabled="isSaving"
                @keydown.enter.prevent="addBucket"
              />
            </div>
          </div>
        </NFormItem>

        <NFormItem
          :label="t('fieldMembers')"
          :validation-status="errorText('MEMBERS_MIN') ? 'error' : undefined"
          :feedback="errorText('MEMBERS_MIN') ?? t('fieldMembersHelp')"
        >
          <div class="protection-dialog__stack">
            <p
              v-if="form.members.length === 0"
              class="protection-dialog__empty"
            >
              {{ t('fieldMembersEmpty') }}
            </p>
            <div
              v-for="(member, index) in form.members"
              :key="member"
              class="protection-dialog__member"
            >
              <NTag round>{{ index + 1 }}</NTag>
              <span class="protection-dialog__member-name">{{ member }}</span>
              <StatusTag
                v-if="index === 0"
                variant="info"
                no-dot
                :label="t('primaryMark')"
              />
              <NButton
                size="tiny"
                :disabled="isSaving || index === 0"
                :aria-label="t('moveUp')"
                :title="t('moveUp')"
                @click="moveMember(index, -1)"
              >
                ↑
              </NButton>
              <NButton
                size="tiny"
                :disabled="isSaving || index === form.members.length - 1"
                :aria-label="t('moveDown')"
                :title="t('moveDown')"
                @click="moveMember(index, 1)"
              >
                ↓
              </NButton>
              <NButton
                size="tiny"
                :disabled="isSaving"
                :aria-label="t('remove')"
                :title="t('remove')"
                @click="removeMemberStorage(member)"
              >
                ✕
              </NButton>
            </div>
            <div class="protection-dialog__inline">
              <NSelect
                v-model:value="selectedStorage"
                :options="availableStorageOptions"
                :placeholder="t('fieldMembersAdd')"
                :disabled="isSaving"
                filterable
                tag
              />
              <NButton
                :disabled="isSaving || !selectedStorage"
                @click="addMemberStorage"
              >
                {{ t('fieldMembersAdd') }}
              </NButton>
            </div>
          </div>
        </NFormItem>
      </template>

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

      <NFormItem
        v-if="!isEdit"
        :show-label="false"
      >
        <div class="protection-dialog__adopt">
          <NCheckbox
            v-model:checked="form.adoptExisting"
            :disabled="isSaving"
          >
            {{ t('fieldAdopt') }}
          </NCheckbox>
          <p class="protection-dialog__adopt-help">{{ t('fieldAdoptHelp') }}</p>
        </div>
      </NFormItem>

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
