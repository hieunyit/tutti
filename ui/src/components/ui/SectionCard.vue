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
  import type { RouteLocationRaw } from 'vue-router';

  withDefaults(
    defineProps<{
      title: string;
      /** no body padding: tables and lists run edge to edge */
      flush?: boolean;
      /** optional header link ("View all") */
      linkLabel?: string;
      linkTo?: RouteLocationRaw;
      /** small grey text next to the title */
      meta?: string;
    }>(),
    {
      flush: false,
      linkLabel: undefined,
      linkTo: undefined,
      meta: undefined,
    },
  );
</script>

<template>
  <section class="tt-section">
    <header class="tt-section__header">
      <h2 class="tt-section__title">{{ title }}</h2>
      <span
        v-if="meta"
        class="tt-section__meta"
        >{{ meta }}</span
      >
      <div
        v-if="linkTo || $slots.actions"
        class="tt-section__actions"
      >
        <slot name="actions">
          <RouterLink
            v-if="linkTo && linkLabel"
            class="tt-section__link"
            :to="linkTo"
            >{{ linkLabel }}</RouterLink
          >
        </slot>
      </div>
    </header>
    <div
      class="tt-section__body"
      :class="{ 'tt-section__body--flush': flush }"
    >
      <slot />
    </div>
  </section>
</template>

<style lang="scss" scoped>
  .tt-section {
    // containing block for absolutely positioned .tt-sr-only cells of inner tables
    position: relative;
    min-width: 0;
    overflow: hidden;
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);

    &__header {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
      min-height: 48px;
      padding: 10px var(--tt-card-padding);
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    &__title {
      margin: 0;
      font-size: 15px;
      line-height: 20px;
      font-weight: 600;
      color: var(--tt-text);
    }

    &__meta {
      flex: 1 1 200px;
      min-width: 0;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__actions {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-left: auto;
    }

    &__link {
      font-size: 13px;
      font-weight: 600;
      color: var(--tt-accent);
      text-decoration: none;

      &:hover {
        color: var(--tt-accent-hover);
        text-decoration: underline;
      }
    }

    &__body {
      padding: var(--tt-card-padding);
      min-width: 0;

      &--flush {
        padding: 0;
        overflow-x: auto;
      }
    }
  }
</style>
