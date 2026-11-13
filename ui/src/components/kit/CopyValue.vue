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
   * Monospace value with a copy button. With `oneTime` (secrets shown once)
   * it adds a warning and an "I have saved this value" tick; the parent keeps
   * its close button disabled until `acknowledged` is emitted with true. The
   * value is only held in memory: never write it to storage or the URL.
   */
  import { onBeforeUnmount, ref, useId } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NCheckbox } from 'naive-ui';
  import i18nKit from './i18nKit';

  const props = withDefaults(
    defineProps<{
      value: string;
      oneTime?: boolean;
      /** accessible name, e.g. "Secret key" */
      label?: string;
    }>(),
    { oneTime: false, label: undefined },
  );

  const emit = defineEmits<{
    (e: 'acknowledged', saved: boolean): void;
    (e: 'copied'): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
  const state = ref<'idle' | 'copied' | 'failed'>('idle');
  const saved = ref(false);
  const warningId = `kit-copy-${useId()}`;
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    try {
      await navigator.clipboard.writeText(props.value);
      state.value = 'copied';
      emit('copied');
    } catch {
      state.value = 'failed';
    }

    clearTimeout(timer);
    timer = setTimeout(() => {
      state.value = 'idle';
    }, 1800);
  }

  function onSaved(value: boolean) {
    saved.value = value;
    emit('acknowledged', value);
  }

  onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <div
    class="kit-copy"
    :class="{ 'kit-copy--one-time': oneTime }"
  >
    <p
      v-if="oneTime"
      :id="warningId"
      class="kit-copy__warning"
      role="note"
    >
      {{ t('oneTimeWarning') }}
    </p>
    <div class="kit-copy__row">
      <code
        class="kit-copy__value"
        :aria-label="label"
        :aria-describedby="oneTime ? warningId : undefined"
        >{{ value }}</code
      >
      <NButton
        size="small"
        :aria-label="label ? `${t('copy')}: ${label}` : t('copy')"
        @click="copy"
      >
        {{
          state === 'copied'
            ? t('copied')
            : state === 'failed'
              ? t('copyFailed')
              : t('copy')
        }}
      </NButton>
    </div>
    <NCheckbox
      v-if="oneTime"
      class="kit-copy__saved"
      :checked="saved"
      @update:checked="onSaved"
    >
      {{ t('savedConfirm') }}
    </NCheckbox>
    <span
      class="tt-sr-only"
      role="status"
      >{{
        state === 'copied'
          ? t('copied')
          : state === 'failed'
            ? t('copyFailed')
            : ''
      }}</span
    >
  </div>
</template>

<style lang="scss" scoped>
  .kit-copy {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);
    min-width: 0;

    &__warning {
      margin: 0;
      padding: var(--tt-space-2) var(--tt-space-3);
      border: 1px solid var(--tt-warning-border);
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-warning-bg);
      color: var(--tt-warning);
      font-size: var(--tt-text-sm);
      line-height: 18px;
    }

    &__row {
      display: flex;
      align-items: center;
      gap: var(--tt-space-2);
      min-width: 0;
    }

    &__value {
      flex: 1 1 auto;
      min-width: 0;
      padding: 6px var(--tt-space-2);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-code-bg);
      font-family: var(--tt-mono);
      font-size: var(--tt-text-sm);
      color: var(--tt-text);
      overflow-wrap: anywhere;
      user-select: all;
    }
  }
</style>
