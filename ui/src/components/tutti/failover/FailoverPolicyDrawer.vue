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
    NAlert,
    NButton,
    NDrawer,
    NDrawerContent,
    NForm,
    NFormItem,
    NInput,
    NInputNumber,
    NSelect,
    NSwitch,
    NTag,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from './i18nFailover';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { FailoverHelper } from '@/utils/helpers/FailoverHelper';
  import type {
    TuttiFailoverPolicy,
    TuttiFailoverPolicyForm,
    TuttiPolicyValidationError,
  } from '@/utils/types/tutti';

  const props = defineProps<{
    show: boolean;
    /** policy being edited; null means a new policy */
    policy: TuttiFailoverPolicy | null;
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved'): void;
  }>();

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { storageNames, isSaving, saveError } = storeToRefs(store);

  function emptyForm(): TuttiFailoverPolicyForm {
    return {
      source: null,
      enabled: true,
      targets: [],
      after: '',
      maxEventLag: null,
    };
  }

  const form = ref<TuttiFailoverPolicyForm>(emptyForm());
  const newTarget = ref<string | null>(null);
  const showErrors = ref(false);

  const isEdit = computed(() => props.policy !== null);

  watch(
    () => props.show,
    (show) => {
      if (!show) return;

      form.value = props.policy
        ? FailoverHelper.policyToForm(props.policy)
        : emptyForm();
      newTarget.value = null;
      showErrors.value = false;
      store.saveError = null;
    },
  );

  const errors = computed<TuttiPolicyValidationError[]>(() =>
    FailoverHelper.validate(form.value),
  );

  function errorsFor(...codes: string[]): string[] {
    return showErrors.value
      ? errors.value.filter((e) => codes.includes(e)).map((e) => t(e))
      : [];
  }

  const sourceOptions = computed(() =>
    storageNames.value.map((name) => ({ label: name, value: name })),
  );

  const targetOptions = computed(() =>
    FailoverHelper.availableTargets(
      storageNames.value,
      form.value.source,
      form.value.targets,
    ).map((name) => ({ label: name, value: name })),
  );

  watch(
    () => form.value.source,
    (source) => {
      form.value.targets = form.value.targets.filter((x) => x !== source);
    },
  );

  function addTarget() {
    if (!newTarget.value) return;

    form.value.targets = FailoverHelper.addTarget(
      form.value.targets,
      newTarget.value,
      form.value.source,
    );
    newTarget.value = null;
  }

  async function submit() {
    showErrors.value = true;

    if (errors.value.length > 0) return;

    if (await store.savePolicy(form.value)) {
      emit('update:show', false);
      emit('saved');
    }
  }
</script>

<template>
  <NDrawer
    :show="show"
    :width="520"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent
      :title="isEdit ? t('editPolicy') : t('addPolicy')"
      closable
    >
      <NForm label-placement="top">
        <NFormItem
          :label="t('fieldSource')"
          :validation-status="
            errorsFor('SOURCE_REQUIRED').length ? 'error' : undefined
          "
          :feedback="errorsFor('SOURCE_REQUIRED')[0]"
        >
          <NSelect
            v-model:value="form.source"
            :options="sourceOptions"
            :disabled="isEdit"
            :placeholder="t('selectSource')"
          />
        </NFormItem>

        <NFormItem :label="t('fieldEnabled')">
          <NSwitch v-model:value="form.enabled" />
        </NFormItem>

        <NFormItem
          :label="t('fieldTargets')"
          :validation-status="
            errorsFor(
              'TARGETS_REQUIRED',
              'TARGET_IS_SOURCE',
              'TARGET_DUPLICATE',
            ).length
              ? 'error'
              : undefined
          "
          :feedback="
            errorsFor(
              'TARGETS_REQUIRED',
              'TARGET_IS_SOURCE',
              'TARGET_DUPLICATE',
            )[0]
          "
        >
          <div class="policy-drawer__targets">
            <p
              v-if="form.targets.length === 0"
              class="policy-drawer__no-targets"
            >
              {{ t('noTargets') }}
            </p>
            <div
              v-for="(target, index) in form.targets"
              :key="target"
              class="policy-drawer__target"
            >
              <NTag round>{{ index + 1 }}</NTag>
              <span class="policy-drawer__target-name">{{ target }}</span>
              <NButton
                size="tiny"
                :disabled="index === 0"
                :aria-label="t('moveUp')"
                @click="
                  form.targets = FailoverHelper.moveTarget(
                    form.targets,
                    index,
                    -1,
                  )
                "
              >
                ↑
              </NButton>
              <NButton
                size="tiny"
                :disabled="index === form.targets.length - 1"
                :aria-label="t('moveDown')"
                @click="
                  form.targets = FailoverHelper.moveTarget(
                    form.targets,
                    index,
                    1,
                  )
                "
              >
                ↓
              </NButton>
              <NButton
                size="tiny"
                :aria-label="t('remove')"
                @click="
                  form.targets = FailoverHelper.removeTarget(
                    form.targets,
                    target,
                  )
                "
              >
                ✕
              </NButton>
            </div>
            <div class="policy-drawer__add-target">
              <NSelect
                v-model:value="newTarget"
                :options="targetOptions"
                :placeholder="t('selectTarget')"
              />
              <NButton
                :disabled="!newTarget"
                @click="addTarget"
              >
                {{ t('addTarget') }}
              </NButton>
            </div>
          </div>
        </NFormItem>

        <NFormItem
          :label="t('fieldAfter')"
          :validation-status="
            errorsFor('AFTER_INVALID').length ? 'error' : undefined
          "
          :feedback="errorsFor('AFTER_INVALID')[0] ?? t('fieldAfterHint')"
        >
          <NInput
            v-model:value="form.after"
            placeholder="60s"
          />
        </NFormItem>

        <NFormItem
          :label="t('fieldMaxEventLag')"
          :validation-status="
            errorsFor('MAX_EVENT_LAG_INVALID').length ? 'error' : undefined
          "
          :feedback="
            errorsFor('MAX_EVENT_LAG_INVALID')[0] ?? t('fieldMaxEventLagHint')
          "
        >
          <NInputNumber
            v-model:value="form.maxEventLag"
            :min="0"
            :precision="0"
            clearable
          />
        </NFormItem>
      </NForm>

      <NAlert
        v-if="saveError"
        type="error"
      >
        {{ saveError === 'unknown' ? t('saveUnknownError') : saveError }}
      </NAlert>

      <template #footer>
        <NButton
          class="policy-drawer__cancel"
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
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style lang="scss" scoped>
  .policy-drawer {
    &__targets {
      width: 100%;
    }

    &__no-targets {
      margin: 0 0 8px;
      opacity: 0.7;
    }

    &__target {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;
    }

    &__target-name {
      flex: 1;
    }

    &__add-target {
      display: flex;
      gap: 8px;
    }

    &__cancel {
      margin-right: 8px;
    }
  }
</style>
