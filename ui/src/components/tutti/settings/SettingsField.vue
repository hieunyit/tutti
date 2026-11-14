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
  /** One labelled setting: label, control (slot), hint or error, restart tag. */
  defineProps<{
    id: string;
    label: string;
    hint?: string;
    error?: string;
    restart?: string;
    /** badge for fields that apply without a restart */
    live?: string;
  }>();
</script>

<template>
  <div class="settings-field">
    <div class="settings-field__head">
      <label
        :for="`settings-${id}`"
        class="settings-field__label"
        >{{ label }}</label
      >
      <span
        v-if="restart"
        class="settings-field__restart"
        data-test="restart-label"
        >{{ restart }}</span
      >
      <span
        v-if="live"
        class="settings-field__live"
        data-test="live-label"
        >{{ live }}</span
      >
    </div>
    <slot />
    <span
      v-if="error || hint"
      :id="`settings-${id}-hint`"
      class="settings-field__hint"
      :class="{ 'settings-field__hint--error': error }"
      >{{ error || hint }}</span
    >
  </div>
</template>

<style lang="scss" scoped>
  .settings-field {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);
    min-width: 0;

    &__head {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }

    &__label {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text);
    }

    &__restart {
      padding: 0 var(--tt-space-2);
      border: 1px solid var(--tt-warning);
      border-radius: var(--tt-radius-pill);
      color: var(--tt-warning);
      font-size: var(--tt-text-xs);
      line-height: 18px;
    }

    &__live {
      padding: 0 var(--tt-space-2);
      border: 1px solid var(--tt-success);
      border-radius: var(--tt-radius-pill);
      color: var(--tt-success);
      font-size: var(--tt-text-xs);
      line-height: 18px;
    }

    &__hint {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);

      &--error {
        color: var(--tt-danger);
      }
    }
  }
</style>
