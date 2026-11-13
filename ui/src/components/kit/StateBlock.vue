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
   * Loading / empty / error placeholder of a page or a table. Use the
   * LoadingBlock, EmptyBlock and ErrorBlock wrappers from '@/components/kit'.
   * The skeleton fades in after 100ms so fast loads never flash a loader.
   */
  import { useI18n } from 'vue-i18n';
  import { NButton, NSkeleton } from 'naive-ui';
  import i18nKit from './i18nKit';

  withDefaults(
    defineProps<{
      kind: 'loading' | 'empty' | 'error';
      title?: string;
      description?: string;
      /** skeleton rows while loading */
      rows?: number;
      /** label of the retry button (error), default "Try again" */
      retryLabel?: string;
    }>(),
    {
      title: undefined,
      description: undefined,
      rows: 4,
      retryLabel: undefined,
    },
  );

  const emit = defineEmits<{
    (e: 'retry'): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
</script>

<template>
  <div
    v-if="kind === 'loading'"
    class="kit-state kit-state--loading"
    data-kit-state="loading"
    aria-busy="true"
    aria-live="polite"
  >
    <span class="tt-sr-only">{{ title ?? t('loading') }}</span>
    <NSkeleton
      class="kit-state__skeleton"
      text
      :repeat="rows"
    />
  </div>

  <div
    v-else-if="kind === 'error'"
    class="kit-state kit-state--error"
    data-kit-state="error"
    role="alert"
  >
    <span
      class="kit-state__icon kit-state__icon--error"
      aria-hidden="true"
      >!</span
    >
    <strong class="kit-state__title">{{ title ?? t('errorTitle') }}</strong>
    <p class="kit-state__text">{{ description ?? t('errorText') }}</p>
    <div class="kit-state__action">
      <NButton
        type="primary"
        @click="emit('retry')"
      >
        {{ retryLabel ?? t('retry') }}
      </NButton>
    </div>
  </div>

  <div
    v-else
    class="kit-state kit-state--empty"
    data-kit-state="empty"
  >
    <strong class="kit-state__title">{{ title ?? t('emptyTitle') }}</strong>
    <p
      v-if="description"
      class="kit-state__text"
    >
      {{ description }}
    </p>
    <div
      v-if="$slots.default"
      class="kit-state__action"
    >
      <slot />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  @keyframes kit-state-appear {
    to {
      opacity: 1;
    }
  }

  .kit-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--tt-space-2);
    padding: var(--tt-space-7) var(--tt-space-4);
    text-align: center;

    &--loading {
      align-items: stretch;
      padding: var(--tt-space-4) 0;
      opacity: 0;
      animation: kit-state-appear var(--tt-duration-fast) linear 100ms forwards;
    }

    &__icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 40px;
      height: 40px;
      border-radius: 50%;
      font-weight: var(--tt-weight-strong);
      font-size: var(--tt-text-lg);

      &--error {
        color: var(--tt-danger);
        background-color: var(--tt-danger-bg);
        border: 1px solid var(--tt-danger-border);
      }
    }

    &__title {
      font-size: var(--tt-text-lg);
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text);
    }

    &__text {
      margin: 0;
      max-width: 56ch;
      font-size: var(--tt-text-md);
      line-height: 20px;
      color: var(--tt-text-2);
    }

    &__action {
      display: flex;
      flex-wrap: wrap;
      justify-content: center;
      gap: var(--tt-space-2);
      margin-top: var(--tt-space-2);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .kit-state--loading {
      animation-duration: 0ms;
    }
  }
</style>
