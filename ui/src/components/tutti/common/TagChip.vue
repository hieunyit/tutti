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
  /** Small square tag: role ("Main"), source ("Managed") and permission labels. */
  withDefaults(
    defineProps<{
      label: string;
      variant?: 'accent' | 'neutral' | 'success' | 'warning' | 'outline';
    }>(),
    { variant: 'neutral' },
  );
</script>

<template>
  <span
    class="tag-chip"
    :class="`tag-chip--${variant}`"
    >{{ label }}</span
  >
</template>

<style lang="scss" scoped>
  .tag-chip {
    display: inline-block;
    padding: 1px 8px;
    font-size: 12px;
    line-height: 18px;
    font-weight: 600;
    white-space: nowrap;
    border-radius: var(--tt-radius-tag);

    &--accent {
      color: var(--tt-accent);
      background-color: var(--tt-accent-subtle);
    }

    &--neutral {
      color: var(--tt-neutral);
      background-color: var(--tt-neutral-bg);
    }

    &--success {
      color: var(--tt-success);
      background-color: var(--tt-success-bg);
    }

    &--warning {
      color: var(--tt-warning);
      background-color: var(--tt-warning-bg);
    }

    &--outline {
      font-weight: 400;
      color: var(--tt-text-2);
      border: 1px solid var(--tt-border-strong);
    }
  }
</style>
