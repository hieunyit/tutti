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
    NRadio,
    NSpace,
    NTag,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import {
    TuttiAccessPermission,
    type TuttiAccessPolicyForm,
    type TuttiAccessPolicyValidationError,
    type TuttiProxyAccount,
  } from '@/utils/types/tutti';

  const props = defineProps<{
    show: boolean;
    account: TuttiProxyAccount | null;
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { isSaving, saveError } = storeToRefs(store);

  const form = ref<TuttiAccessPolicyForm>(AccessPolicyHelper.policyToForm(''));
  const newPattern = ref('');
  const showErrors = ref(false);

  const permissions = computed(() => [
    {
      value: TuttiAccessPermission.READ_WRITE,
      label: t('permissionReadWrite'),
      hint: t('permissionReadWriteHint'),
    },
    {
      value: TuttiAccessPermission.READ_ONLY,
      label: t('permissionReadOnly'),
      hint: t('permissionReadOnlyHint'),
    },
    {
      value: TuttiAccessPermission.WRITE_ONLY,
      label: t('permissionWriteOnly'),
      hint: t('permissionWriteOnlyHint'),
    },
  ]);

  watch(
    () => props.show,
    (show) => {
      if (!show || !props.account) return;

      form.value = AccessPolicyHelper.policyToForm(
        props.account.user,
        props.account.policy,
      );
      newPattern.value = '';
      showErrors.value = false;
      store.saveError = null;
    },
  );

  /** the not-yet-added input counts too so a typo is reported before saving */
  const patternError = computed<TuttiAccessPolicyValidationError | null>(() =>
    newPattern.value === ''
      ? null
      : AccessPolicyHelper.validatePattern(newPattern.value),
  );

  const errors = computed(() => AccessPolicyHelper.validate(form.value));

  const listErrors = computed(() => {
    const codes = errors.value.filter((e) => e.startsWith('PATTERN_'));
    const all = patternError.value ? [...codes, patternError.value] : codes;

    return showErrors.value || patternError.value
      ? [...new Set(all)].map((e) => t(e))
      : [];
  });

  function addPattern() {
    if (newPattern.value === '' || patternError.value) return;

    form.value.buckets = AccessPolicyHelper.addPattern(
      form.value.buckets,
      newPattern.value,
    );
    newPattern.value = '';
  }

  async function submit() {
    showErrors.value = true;

    // a pending, valid pattern is added implicitly
    if (newPattern.value !== '') {
      if (patternError.value) return;

      addPattern();
    }

    if (AccessPolicyHelper.validate(form.value).length > 0) return;

    if (await store.savePolicy(form.value)) {
      emit('update:show', false);
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
      :title="account?.hasPolicy ? t('editPolicy') : t('createPolicy')"
      closable
    >
      <NForm label-placement="top">
        <NFormItem :label="t('fieldUser')">
          <NInput
            :value="form.user"
            disabled
          />
        </NFormItem>

        <NFormItem :label="t('fieldPermission')">
          <NSpace
            vertical
            size="small"
          >
            <NRadio
              v-for="permission in permissions"
              :key="permission.value"
              :checked="form.permission === permission.value"
              :value="permission.value"
              @update:checked="form.permission = permission.value"
            >
              <strong>{{ permission.label }}</strong>
              <div class="policy-drawer__hint">{{ permission.hint }}</div>
            </NRadio>
          </NSpace>
        </NFormItem>

        <NFormItem
          :label="t('fieldBuckets')"
          :validation-status="listErrors.length ? 'error' : undefined"
          :feedback="listErrors[0] ?? t('fieldBucketsHint')"
        >
          <div class="policy-drawer__patterns">
            <p
              v-if="form.buckets.length === 0"
              class="policy-drawer__hint"
            >
              {{ t('noPatterns') }}
            </p>
            <div
              v-for="pattern in form.buckets"
              :key="pattern"
              class="policy-drawer__pattern"
            >
              <NTag round>{{ pattern }}</NTag>
              <NButton
                size="tiny"
                :aria-label="t('removePattern')"
                @click="
                  form.buckets = AccessPolicyHelper.removePattern(
                    form.buckets,
                    pattern,
                  )
                "
              >
                ✕
              </NButton>
            </div>
            <div class="policy-drawer__add">
              <NInput
                v-model:value="newPattern"
                :placeholder="t('bucketPlaceholder')"
                @keydown.enter.prevent="addPattern"
              />
              <NButton
                :disabled="newPattern === '' || patternError !== null"
                @click="addPattern"
              >
                {{ t('addPattern') }}
              </NButton>
            </div>
          </div>
        </NFormItem>

        <NFormItem :label="t('fieldDescription')">
          <NInput
            v-model:value="form.description"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
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
    &__hint {
      margin: 0;
      font-size: 12px;
      opacity: 0.7;
    }

    &__patterns {
      width: 100%;
    }

    &__pattern {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;
    }

    &__add {
      display: flex;
      gap: 8px;
    }

    &__cancel {
      margin-right: 8px;
    }
  }
</style>
