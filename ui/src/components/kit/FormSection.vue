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
   * A titled group of form fields (a fieldset): title, short description,
   * the fields in the default slot, and an inline section error. Fields keep
   * visible labels (NFormItem) and validate on blur, not on every keystroke.
   */
  import { useId } from 'vue';

  withDefaults(
    defineProps<{
      title: string;
      description?: string;
      /** section-level error, e.g. a server rejection for this group */
      error?: string;
      /** tag shown next to the title, e.g. "Needs worker restart" */
      tag?: string;
    }>(),
    { description: undefined, error: undefined, tag: undefined },
  );

  const uid = `kit-form-${useId()}`;
</script>

<template>
  <fieldset
    class="kit-form"
    :aria-describedby="description ? `${uid}-desc` : undefined"
    :aria-invalid="error ? 'true' : undefined"
  >
    <legend class="kit-form__legend">
      <span class="kit-form__title">{{ title }}</span>
      <span
        v-if="tag"
        class="kit-form__tag"
        >{{ tag }}</span
      >
    </legend>
    <p
      v-if="description"
      :id="`${uid}-desc`"
      class="kit-form__description"
    >
      {{ description }}
    </p>
    <div class="kit-form__fields">
      <slot />
    </div>
    <p
      v-if="error"
      class="kit-form__error"
      role="alert"
    >
      {{ error }}
    </p>
  </fieldset>
</template>

<style lang="scss" scoped>
  .kit-form {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    min-width: 0;
    margin: 0;
    padding: var(--tt-space-5);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);
    background-color: var(--tt-surface);

    // float keeps the legend inside the box instead of on the border
    &__legend {
      float: left;
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      width: 100%;
      padding: 0;
    }

    &__title {
      font-size: var(--tt-text-lg);
      line-height: 24px;
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text);
    }

    &__tag {
      align-self: flex-start;
      padding: 0 var(--tt-space-2);
      border: 1px solid var(--tt-warning-border);
      border-radius: var(--tt-radius-pill);
      background-color: var(--tt-warning-bg);
      font-size: var(--tt-text-xs);
      line-height: 18px;
      color: var(--tt-warning);
    }

    &__description {
      margin: calc(var(--tt-space-2) * -1) 0 0;
      max-width: var(--tt-measure);
      font-size: var(--tt-text-sm);
      line-height: 18px;
      color: var(--tt-text-2);
    }

    &__fields {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
    }

    &__error {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-danger);
    }
  }
</style>
