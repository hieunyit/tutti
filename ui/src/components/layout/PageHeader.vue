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
  // Legacy page header; new pages use PageShell from '@/components/kit'.
  defineProps<{
    title: string;
    description?: string;
    /** "Live · updated 4 s ago": shown with a status dot before the actions */
    liveText?: string;
  }>();
</script>

<template>
  <header class="page-header">
    <div class="page-header__row">
      <div class="page-header__text">
        <h1 class="page-header__title">{{ title }}</h1>
        <p
          v-if="description"
          class="page-header__description"
        >
          {{ description }}
        </p>
      </div>

      <span
        v-if="liveText || $slots.live"
        class="page-header__live"
        aria-live="polite"
      >
        <slot name="live">
          <span
            class="page-header__live-dot"
            aria-hidden="true"
          />
          {{ liveText }}
        </slot>
      </span>

      <div
        v-if="$slots.actions"
        class="page-header__actions"
      >
        <slot name="actions" />
      </div>
    </div>

    <div
      v-if="$slots.extra"
      class="page-header__extra"
    >
      <slot name="extra" />
    </div>
  </header>
</template>

<style lang="scss" scoped>
  .page-header {
    margin-bottom: var(--tt-space-5);

    &__row {
      display: flex;
      align-items: flex-end;
      flex-wrap: wrap;
      gap: 12px 16px;
    }

    &__text {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 0;
      flex: 1 1 360px;
    }

    &__title {
      margin: 0;
      font-size: var(--tt-text-2xl);
      line-height: 32px;
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text);
    }

    &__description {
      margin: 0;
      font-size: 14px;
      line-height: 20px;
      color: var(--tt-text-2);
      max-width: var(--tt-measure);
    }

    &__live {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
      color: var(--tt-text-3);
    }

    &__live-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: var(--tt-success-solid);
    }

    &__actions {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
      margin-left: auto;
    }

    &__extra {
      margin-top: 16px;
    }
  }
</style>
