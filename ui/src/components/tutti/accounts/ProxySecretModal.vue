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
  import { NAlert, NButton, NInput, NModal, NSpace } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import CopyButton from '@/components/chorus/common/CopyButton/CopyButton.vue';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';

  /*
   * One-time display of a secret. The secret only lives in the props of this
   * component; it is not stored, logged or persisted anywhere.
   */
  const props = defineProps<{
    secret: TuttiProxyAccountSecret | null;
    kind: 'created' | 'rotated';
  }>();

  const emit = defineEmits<{
    (e: 'close'): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });

  const expiry = computed(() =>
    ProxyAccountHelper.hasTime(props.secret?.previousSecretExpiresAt)
      ? new Date(
          props.secret?.previousSecretExpiresAt as string,
        ).toLocaleString()
      : null,
  );
</script>

<template>
  <NModal
    :show="secret !== null"
    preset="card"
    :title="
      kind === 'created' ? t('secretTitleCreated') : t('secretTitleRotated')
    "
    :style="{ width: '560px', maxWidth: '95vw' }"
    :mask-closable="false"
    :close-on-esc="false"
    :closable="false"
  >
    <NSpace
      v-if="secret"
      vertical
      :size="12"
    >
      <NAlert type="warning">
        {{ t('secretWarning') }}
      </NAlert>

      <div>
        <div>{{ t('secretAccessKeyId') }}</div>
        <NSpace
          align="center"
          :wrap="false"
        >
          <NInput
            readonly
            :value="secret.accessKeyId"
            :style="{ minWidth: '320px' }"
          />
          <CopyButton :text="secret.accessKeyId" />
        </NSpace>
      </div>

      <div>
        <div>{{ t('secretSecret') }}</div>
        <NSpace
          align="center"
          :wrap="false"
        >
          <NInput
            readonly
            type="password"
            show-password-on="click"
            autocomplete="off"
            :value="secret.secretAccessKey ?? ''"
            :style="{ minWidth: '320px' }"
          />
          <CopyButton :text="secret.secretAccessKey ?? ''" />
        </NSpace>
      </div>

      <NAlert
        v-if="expiry"
        type="info"
      >
        {{ t('secretPreviousExpires', { time: expiry }) }}
      </NAlert>
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton
          type="primary"
          @click="emit('close')"
        >
          {{ t('secretConfirm') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
