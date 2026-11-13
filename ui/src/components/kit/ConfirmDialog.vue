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
  /**
   * Confirmation of a (destructive) action. Names the target, uses a red
   * button when `danger`, and with `requireTyping` keeps the confirm button
   * disabled until the exact target name is typed. The default slot holds
   * the consequence text. The label is a verb ("Delete storage"), never "OK".
   */
  import { computed, ref, useId, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput, NModal } from 'naive-ui';
  import i18nKit from './i18nKit';

  const props = withDefaults(
    defineProps<{
      open: boolean;
      title: string;
      targetName?: string;
      requireTyping?: boolean;
      danger?: boolean;
      confirmLabel: string;
      /** confirm in progress: spinner, cancel disabled */
      loading?: boolean;
    }>(),
    {
      targetName: undefined,
      requireTyping: false,
      danger: false,
      loading: false,
    },
  );

  const emit = defineEmits<{
    (e: 'confirm'): void;
    (e: 'cancel'): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
  const typed = ref('');
  const inputId = `kit-confirm-${useId()}`;

  const needsTyping = computed(
    () => props.requireTyping && Boolean(props.targetName),
  );
  const confirmDisabled = computed(
    () =>
      props.loading ||
      (needsTyping.value && typed.value.trim() !== props.targetName),
  );

  watch(
    () => props.open,
    (open) => {
      if (open) typed.value = '';
    },
  );

  function confirm() {
    if (confirmDisabled.value) return;

    emit('confirm');
  }

  function onShowChange(show: boolean) {
    if (!show && !props.loading) emit('cancel');
  }
</script>

<template>
  <NModal
    :show="open"
    preset="card"
    role="alertdialog"
    :title="title"
    :closable="!loading"
    :mask-closable="!loading"
    :style="{ width: '460px', maxWidth: 'calc(100vw - 32px)' }"
    @update:show="onShowChange"
  >
    <div class="kit-confirm">
      <div
        v-if="$slots.default"
        class="kit-confirm__text"
      >
        <slot />
      </div>
      <p
        v-if="targetName"
        class="kit-confirm__target"
      >
        {{ t('affects') }}
        <strong class="tt-mono">{{ targetName }}</strong>
      </p>
      <div
        v-if="needsTyping"
        class="kit-confirm__typing"
      >
        <label
          class="kit-confirm__label"
          :for="inputId"
          >{{ t('typeToConfirm', { name: targetName }) }}</label
        >
        <NInput
          v-model:value="typed"
          data-kit-confirm-input
          :input-props="{
            id: inputId,
            autocomplete: 'off',
            spellcheck: false,
          }"
          :disabled="loading"
          @keydown.enter.prevent="confirm"
        />
      </div>
    </div>

    <template #footer>
      <div class="kit-confirm__actions">
        <NButton
          data-kit-cancel
          :disabled="loading"
          @click="emit('cancel')"
        >
          {{ t('cancel') }}
        </NButton>
        <NButton
          data-kit-confirm
          :type="danger ? 'error' : 'primary'"
          :disabled="confirmDisabled"
          :loading="loading"
          @click="confirm"
        >
          {{ confirmLabel }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style lang="scss" scoped>
  .kit-confirm {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    font-size: var(--tt-text-md);
    line-height: 20px;
    color: var(--tt-text-2);

    &__text {
      max-width: var(--tt-measure);
    }

    &__target {
      margin: 0;

      strong {
        color: var(--tt-text);
        font-weight: var(--tt-weight-strong);
        overflow-wrap: anywhere;
      }
    }

    &__typing {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
    }

    &__label {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text);
    }

    &__actions {
      display: flex;
      justify-content: flex-end;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }
  }
</style>
