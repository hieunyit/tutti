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
  import { NButton, NModal } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import { Callout, CopyValue } from '@/components/kit';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';

  /*
   * One-time display of a secret. The secret only lives in the props of this
   * component; it is not stored, logged or persisted anywhere. The dialog
   * cannot be dismissed (outside click, Esc) until the user confirmed that
   * the secret was stored.
   */
  const props = defineProps<{
    secret: TuttiProxyAccountSecret | null;
    kind: 'created' | 'rotated';
    /** proxy address for the AWS_ENDPOINT_URL line of the .env download */
    endpoint?: string;
  }>();

  const emit = defineEmits<{
    (e: 'close'): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const stored = ref(false);

  watch(
    () => props.secret,
    () => {
      stored.value = false;
    },
  );

  const expiry = computed(() =>
    ProxyAccountHelper.hasTime(props.secret?.previousSecretExpiresAt)
      ? formatDateTime(props.secret?.previousSecretExpiresAt)
      : null,
  );

  /** Builds the file in the browser; nothing is sent or stored. */
  function downloadEnv() {
    if (!props.secret) return;

    const content = ProxyAccountHelper.buildEnvFile(
      props.secret.accessKeyId,
      props.secret.secretAccessKey ?? '',
      props.endpoint || 'http://<proxy-address>',
    );
    const url = URL.createObjectURL(
      new Blob([content], { type: 'text/plain;charset=utf-8' }),
    );
    const link = document.createElement('a');

    link.href = url;
    link.download = `${props.secret.accessKeyId}.env`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }
</script>

<template>
  <NModal
    :show="secret !== null"
    preset="card"
    :title="
      kind === 'created' ? t('secretTitleCreated') : t('secretTitleRotated')
    "
    role="dialog"
    :style="{ width: '540px', maxWidth: '95vw' }"
    :mask-closable="false"
    :close-on-esc="false"
    :closable="false"
  >
    <div
      v-if="secret"
      class="secret-modal"
    >
      <div class="secret-modal__field">
        <span class="secret-modal__label">{{ t('secretAccessKeyId') }}</span>
        <CopyValue
          :value="secret.accessKeyId"
          :label="t('secretAccessKeyId')"
        />
      </div>

      <div class="secret-modal__field">
        <span class="secret-modal__label">{{ t('secretSecret') }}</span>
        <CopyValue
          :key="secret.accessKeyId"
          :value="secret.secretAccessKey ?? ''"
          :label="t('secretSecret')"
          one-time
          @acknowledged="stored = $event"
        />
      </div>

      <Callout
        v-if="expiry"
        tone="info"
      >
        {{ t('secretPreviousExpires', { time: expiry }) }}
      </Callout>

      <div class="secret-modal__env">
        <NButton
          secondary
          @click="downloadEnv"
        >
          {{ t('secretDownloadEnv') }}
        </NButton>
        <span class="secret-modal__hint">{{ t('secretEnvHint') }}</span>
      </div>
    </div>

    <template #footer>
      <div class="secret-modal__footer">
        <NButton
          type="primary"
          :disabled="!stored"
          @click="emit('close')"
        >
          {{ t('secretConfirm') }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style lang="scss" scoped>
  .secret-modal {
    display: flex;
    flex-direction: column;
    gap: 14px;

    &__field {
      display: flex;
      flex-direction: column;
      gap: 6px;
      min-width: 0;
    }

    &__label {
      font-size: 13px;
      font-weight: 500;
    }

    &__env {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px 12px;
    }

    &__hint {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__footer {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>
