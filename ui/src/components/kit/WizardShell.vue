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
   * Multi-step wizard. Each step renders the slot named after its key; Next
   * stays disabled until the step is `valid`; Back/Next/Finish sit in a sticky
   * footer. The current step is kept per `draftKey` in sessionStorage, and the
   * form data should live in `useWizardDraft(draftKey, ...)`, so leaving and
   * coming back restores both. "Start over" clears them and emits `reset`; it
   * only shows after the first step (nothing to start over on step 1).
   * Shows "Step n of m" with a progress bar, and an optional `summary` slot
   * (an aside next to the step, stacked below it on narrow screens) that
   * phase 2 wizards use for the running summary.
   */
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import {
    clearWizardDraft,
    readWizardStep,
    writeWizardStep,
  } from './composables';
  import i18nKit from './i18nKit';
  import type { WizardStep } from './types';

  const props = withDefaults(
    defineProps<{
      steps: WizardStep[];
      draftKey: string;
      /** verb label of the last button, e.g. "Create replication" */
      finishLabel?: string;
      /** finish in progress */
      busy?: boolean;
      /** keep "Start over" on step 1 too (default: only after step 1) */
      startOverOnFirstStep?: boolean;
    }>(),
    { finishLabel: undefined, busy: false, startOverOnFirstStep: false },
  );

  const emit = defineEmits<{
    (e: 'finish'): void;
    (e: 'reset'): void;
    (e: 'step', key: string): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
  const current = ref(clampStep(readWizardStep(props.draftKey)));

  const step = computed(() => props.steps[current.value]);
  const isLast = computed(() => current.value === props.steps.length - 1);
  const progressText = computed(() =>
    t('stepOf', { current: current.value + 1, total: props.steps.length }),
  );

  /** never past the first invalid step (e.g. a restored, now stale draft) */
  function clampStep(index: number) {
    const firstInvalid = props.steps.findIndex((s) => !s.valid);
    const limit = firstInvalid >= 0 ? firstInvalid : props.steps.length - 1;

    return Math.max(0, Math.min(index, limit));
  }

  /** a step is reachable when every step before it is valid */
  function reachable(index: number) {
    return props.steps.slice(0, index).every((s) => s.valid);
  }

  function go(index: number) {
    if (index < 0 || index >= props.steps.length || !reachable(index)) return;

    current.value = index;
  }

  function next() {
    if (!step.value?.valid || props.busy) return;

    if (isLast.value) {
      emit('finish');

      return;
    }

    go(current.value + 1);
  }

  function startOver() {
    clearWizardDraft(props.draftKey);
    current.value = 0;
    emit('reset');
  }

  watch(current, (index) => {
    writeWizardStep(props.draftKey, index);

    const key = props.steps[index]?.key;

    if (key) emit('step', key);
  });

  // a step that became invalid sends the user back to it
  watch(
    () => props.steps.map((s) => s.valid),
    () => {
      current.value = clampStep(current.value);
    },
  );
</script>

<template>
  <div class="kit-wizard">
    <ol class="kit-wizard__steps">
      <li
        v-for="(s, index) in steps"
        :key="s.key"
        class="kit-wizard__step"
        :class="{
          'kit-wizard__step--current': index === current,
          'kit-wizard__step--done': index < current,
        }"
      >
        <button
          type="button"
          class="kit-wizard__step-button"
          :disabled="index === current || !reachable(index)"
          :aria-current="index === current ? 'step' : undefined"
          @click="go(index)"
        >
          <span
            class="kit-wizard__step-index"
            aria-hidden="true"
            >{{ index + 1 }}</span
          >
          <span class="kit-wizard__step-label">{{ s.label }}</span>
        </button>
      </li>
    </ol>

    <div
      class="kit-wizard__progress"
      role="progressbar"
      aria-valuemin="1"
      :aria-valuemax="steps.length"
      :aria-valuenow="current + 1"
      :aria-valuetext="progressText"
    >
      <span
        class="kit-wizard__progress-fill"
        :style="{ width: `${((current + 1) / steps.length) * 100}%` }"
      />
    </div>
    <p class="kit-wizard__progress-text">{{ progressText }}</p>

    <div
      class="kit-wizard__content"
      :class="{ 'kit-wizard__content--aside': $slots.summary }"
    >
      <div class="kit-wizard__body">
        <template v-if="step">
          <slot :name="step.key" />
        </template>
      </div>
      <aside
        v-if="$slots.summary"
        class="kit-wizard__summary"
        :aria-label="t('summary')"
      >
        <slot
          name="summary"
          :step="step?.key"
        />
      </aside>
    </div>

    <footer class="kit-wizard__footer">
      <NButton
        v-if="current > 0 || startOverOnFirstStep"
        quaternary
        :disabled="busy"
        @click="startOver"
      >
        {{ t('startOver') }}
      </NButton>
      <div class="kit-wizard__nav">
        <NButton
          v-if="current > 0"
          :disabled="busy"
          @click="go(current - 1)"
        >
          {{ t('back') }}
        </NButton>
        <NButton
          type="primary"
          :disabled="!step?.valid"
          :loading="busy && isLast"
          @click="next"
        >
          {{ isLast ? (finishLabel ?? t('finish')) : t('next') }}
        </NButton>
      </div>
    </footer>
  </div>
</template>

<style lang="scss" scoped>
  .kit-wizard {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);

    &__steps {
      display: flex;
      gap: var(--tt-space-2);
      margin: 0;
      padding: 0;
      list-style: none;
      overflow-x: auto;
    }

    &__step {
      flex: 1 1 0;
      min-width: 120px;
      border-top: 3px solid var(--tt-border);

      &--done {
        border-top-color: var(--tt-success-solid);
      }

      &--current {
        border-top-color: var(--tt-accent);
      }
    }

    &__step-button {
      display: flex;
      align-items: center;
      gap: var(--tt-space-2);
      width: 100%;
      padding: var(--tt-space-2) 0;
      border: 0;
      background: transparent;
      font: inherit;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
      text-align: left;
      cursor: pointer;

      &:disabled {
        cursor: default;
      }

      .kit-wizard__step--current & {
        color: var(--tt-text);
        font-weight: var(--tt-weight-strong);
      }
    }

    &__step-index {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 24px;
      height: 24px;
      border: 1px solid var(--tt-border-strong);
      border-radius: 50%;
      font-size: var(--tt-text-xs);
    }

    /* the step header already shows the progress: keep the bar for screen readers only */
    &__progress {
      position: absolute;
      width: 1px;
      height: 1px;
      margin: -1px;
      overflow: hidden;
      clip: rect(0 0 0 0);
    }

    &__progress-fill {
      display: block;
      height: 100%;
      background-color: var(--tt-accent);
      transition: width var(--tt-duration) var(--tt-ease);
    }

    &__progress-text {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__content {
      display: grid;
      gap: var(--tt-space-5);

      &--aside {
        grid-template-columns: minmax(0, 1fr) minmax(240px, 320px);
        align-items: start;

        @media screen and (max-width: 1023px) {
          grid-template-columns: minmax(0, 1fr);
        }
      }
    }

    &__summary {
      padding: var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-subtle);
    }

    &__body {
      min-width: 0;
    }

    &__footer {
      position: sticky;
      bottom: 0;
      z-index: 2;
      display: flex;
      align-items: center;
      gap: var(--tt-space-2);
      padding: var(--tt-space-3) 0;
      border-top: 1px solid var(--tt-border);
      background-color: var(--tt-bg);
    }

    &__nav {
      display: flex;
      gap: var(--tt-space-2);
      margin-left: auto;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .kit-wizard__progress-fill {
      transition: none;
    }
  }
</style>
