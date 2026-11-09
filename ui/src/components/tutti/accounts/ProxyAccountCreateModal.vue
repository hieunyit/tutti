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
    NForm,
    NFormItem,
    NInput,
    NModal,
    NSelect,
    NSpace,
  } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';

  const props = defineProps<{
    show: boolean;
    users: string[];
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'created', secret: TuttiProxyAccountSecret): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();

  const user = ref<string | null>(null);
  const description = ref('');
  const isSaving = ref(false);
  const errorMessage = ref<string | null>(null);

  const options = computed(() =>
    props.users.map((name) => ({ label: name, value: name })),
  );

  watch(
    () => props.show,
    (show) => {
      if (!show) return;

      user.value = null;
      description.value = '';
      errorMessage.value = null;
    },
  );

  async function submit() {
    if (!user.value) {
      errorMessage.value = t('createUserRequired');

      return;
    }

    isSaving.value = true;
    errorMessage.value = null;

    try {
      const secret = await store.createAccount(user.value, description.value);

      emit('update:show', false);
      emit('created', secret);
    } catch (error: unknown) {
      errorMessage.value = ProxyAccountHelper.isNotConfigured(error)
        ? t('notConfigured')
        : (ProxyAccountHelper.getMessage(error) ?? t('actionUnknownError'));
    } finally {
      isSaving.value = false;
    }
  }
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('createAccountTitle')"
    :style="{ width: '480px', maxWidth: '95vw' }"
    :mask-closable="false"
    @update:show="emit('update:show', $event)"
  >
    <p>{{ t('createAccountHint') }}</p>

    <NForm label-placement="top">
      <NFormItem :label="t('fieldUser')">
        <NSelect
          v-model:value="user"
          filterable
          :options="options"
          :placeholder="t('createUserPlaceholder')"
          :disabled="isSaving"
        />
      </NFormItem>
      <NFormItem :label="t('fieldDescription')">
        <NInput
          v-model:value="description"
          :placeholder="t('createDescriptionPlaceholder')"
          :disabled="isSaving"
        />
      </NFormItem>
    </NForm>

    <NAlert
      v-if="errorMessage"
      type="error"
    >
      {{ errorMessage }}
    </NAlert>

    <template #footer>
      <NSpace justify="end">
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
          {{ t('create') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
