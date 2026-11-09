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
    NModal,
    NRadio,
    NRadioGroup,
    NSpace,
  } from 'naive-ui';
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';

  const props = defineProps<{
    /** access key id to rotate; null keeps the dialog closed */
    accessKeyId: string | null;
  }>();

  const emit = defineEmits<{
    (e: 'close'): void;
    (e: 'rotated', secret: TuttiProxyAccountSecret): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();

  const grace = ref<string>(ProxyAccountHelper.DEFAULT_GRACE_ID);
  const isSaving = ref(false);
  const errorMessage = ref<string | null>(null);

  const labelKeys: Record<string, string> = {
    immediate: 'graceImmediate',
    hour: 'graceHour',
    day: 'graceDay',
    week: 'graceWeek',
  };

  watch(
    () => props.accessKeyId,
    () => {
      grace.value = ProxyAccountHelper.DEFAULT_GRACE_ID;
      errorMessage.value = null;
    },
  );

  async function submit() {
    if (!props.accessKeyId) return;

    isSaving.value = true;
    errorMessage.value = null;

    try {
      const secret = await store.rotateSecret(
        props.accessKeyId,
        ProxyAccountHelper.getGraceValue(grace.value),
      );

      emit('rotated', secret);
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
    :show="accessKeyId !== null"
    preset="card"
    :title="t('rotateTitle')"
    :style="{ width: '460px', maxWidth: '95vw' }"
    :mask-closable="false"
    @update:show="(v: boolean) => !v && emit('close')"
  >
    <NSpace
      vertical
      :size="12"
    >
      <p>{{ t('rotateHint', { key: accessKeyId ?? '' }) }}</p>

      <div>{{ t('rotateGrace') }}</div>
      <NRadioGroup
        v-model:value="grace"
        :disabled="isSaving"
      >
        <NSpace vertical>
          <NRadio
            v-for="option in ProxyAccountHelper.GRACE_OPTIONS"
            :key="option.id"
            :value="option.id"
          >
            {{ t(labelKeys[option.id] ?? 'graceHour') }}
          </NRadio>
        </NSpace>
      </NRadioGroup>

      <NAlert
        v-if="errorMessage"
        type="error"
      >
        {{ errorMessage }}
      </NAlert>
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton
          :disabled="isSaving"
          @click="emit('close')"
        >
          {{ t('cancel') }}
        </NButton>
        <NButton
          type="primary"
          :loading="isSaving"
          @click="submit"
        >
          {{ t('rotate') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
