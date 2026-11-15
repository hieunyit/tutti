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
   * The one status badge: colored dot plus text, the label carries the meaning,
   * not the color. Give a `tone`, or a domain `status` word ("healthy", "paused")
   * that is mapped to a tone; the default slot overrides `label`. `noDot` hides
   * the dot. Colors come only from the --tt-* semantic tokens.
   */
  import { computed } from 'vue';
  import type { StatusTone } from './types';
  import { mapStatusToVariant } from '@/components/ui/status';

  const props = withDefaults(
    defineProps<{
      tone?: StatusTone;
      /** domain status word, mapped with mapStatusToVariant when no tone is given */
      status?: string | null;
      label?: string;
      noDot?: boolean;
    }>(),
    { tone: undefined, status: undefined, label: undefined, noDot: false },
  );

  const resolved = computed<StatusTone>(
    () => props.tone ?? mapStatusToVariant(props.status),
  );
</script>

<template>
  <span
    class="kit-status"
    :class="`kit-status--${resolved}`"
    :data-tone="resolved"
  >
    <span
      v-if="!noDot"
      class="kit-status__dot"
      aria-hidden="true"
    />
    <slot>{{ label }}</slot>
  </span>
</template>

<style lang="scss" scoped>
  .kit-status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 100%;
    padding: 1px var(--tt-space-2);
    border: 1px solid var(--kit-status-border);
    border-radius: var(--tt-radius-pill);
    background-color: var(--kit-status-bg);
    color: var(--kit-status-text);
    font-size: var(--tt-text-xs);
    line-height: 18px;
    font-weight: var(--tt-weight-medium);
    white-space: nowrap;

    &__dot {
      flex: none;
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background-color: var(--kit-status-dot);
    }

    @each $tone in success, warning, danger, info, neutral {
      &--#{$tone} {
        --kit-status-text: var(--tt-#{$tone});
        --kit-status-bg: var(--tt-#{$tone}-bg);
        --kit-status-border: var(--tt-#{$tone}-border);
        --kit-status-dot: var(--tt-#{$tone}-solid);
      }
    }
  }
</style>
