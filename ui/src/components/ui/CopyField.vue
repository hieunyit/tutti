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
  import { computed, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nUi from './i18nUi';

  const props = withDefaults(
    defineProps<{
      value: string;
      /** accessible name of the value and the button, e.g. "S3 endpoint" */
      label?: string;
      /** mask the value until "Show" is pressed; copying always copies the real value */
      secret?: boolean;
    }>(),
    { label: undefined, secret: false },
  );

  const { t } = useI18n({ messages: i18nUi });
  const state = ref<'idle' | 'copied' | 'failed'>('idle');
  const revealed = ref(false);
  const shown = computed(() =>
    props.secret && !revealed.value ? '•'.repeat(24) : props.value,
  );
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    try {
      await navigator.clipboard.writeText(props.value);
      state.value = 'copied';
    } catch {
      state.value = 'failed';
    }

    clearTimeout(timer);
    timer = setTimeout(() => {
      state.value = 'idle';
    }, 1800);
  }

  onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <span class="tt-copy">
    <code
      class="tt-copy__value"
      :aria-label="label"
      >{{ shown }}</code
    >
    <button
      v-if="secret"
      type="button"
      class="tt-copy__button"
      :aria-pressed="revealed"
      :aria-label="
        label ? `${revealed ? t('hide') : t('show')}: ${label}` : undefined
      "
      @click="revealed = !revealed"
    >
      {{ revealed ? t('hide') : t('show') }}
    </button>
    <button
      type="button"
      class="tt-copy__button"
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
    </button>
    <span
      class="tt-copy__live"
      role="status"
      >{{
        state === 'copied'
          ? t('copied')
          : state === 'failed'
            ? t('copyFailed')
            : ''
      }}</span
    >
  </span>
</template>

<style lang="scss" scoped>
  .tt-copy {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    max-width: 100%;
    min-width: 0;

    &__value {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      padding: 8px 12px;
      font-family: var(--tt-mono);
      font-size: 13px;
      color: var(--tt-text);
      background-color: var(--tt-code-bg);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
    }

    &__button {
      flex: none;
      height: 36px;
      padding: 0 14px;
      font: inherit;
      font-size: 14px;
      font-weight: 500;
      color: var(--tt-text);
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
      cursor: pointer;

      &:hover {
        background-color: var(--tt-subtle);
      }
    }

    &__live {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
  }
</style>
