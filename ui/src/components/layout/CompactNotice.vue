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
   * Phone-only one-line notice (replaces the stacked warning callouts).
   * Dismissal is remembered for the browser session only.
   */
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import type { RouteLocationRaw } from 'vue-router';
  import i18nLayout from './i18nLayout';

  const props = defineProps<{
    /** stable id, used for the session dismissal */
    id: string;
    text: string;
    linkLabel: string;
    to: RouteLocationRaw;
  }>();

  const { t } = useI18n({ messages: i18nLayout });
  const storageKey = computed(() => `tutti.notice.dismissed.${props.id}`);

  function wasDismissed(): boolean {
    try {
      return sessionStorage.getItem(storageKey.value) === '1';
    } catch {
      return false;
    }
  }

  const dismissed = ref(wasDismissed());

  function dismiss() {
    dismissed.value = true;

    try {
      sessionStorage.setItem(storageKey.value, '1');
    } catch {
      // storage unavailable: dismissed until the page reloads
    }
  }
</script>

<template>
  <div
    v-if="!dismissed"
    class="compact-notice"
    role="status"
  >
    <span
      class="compact-notice__text"
      :title="text"
      >{{ text }}</span
    >
    <RouterLink
      class="compact-notice__link"
      :to="to"
      >{{ linkLabel }}</RouterLink
    >
    <button
      type="button"
      class="compact-notice__close"
      :aria-label="t('dismissNotice')"
      @click="dismiss"
    >
      <svg
        aria-hidden="true"
        width="12"
        height="12"
        viewBox="0 0 12 12"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
      >
        <path d="M2 2l8 8M10 2l-8 8" />
      </svg>
    </button>
  </div>
</template>

<style lang="scss" scoped>
  .compact-notice {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 32px;
    box-sizing: border-box;
    padding: 4px 8px 4px 12px;
    border-top: 1px solid var(--tt-warning-border);
    background-color: var(--tt-warning-bg);
    color: var(--tt-text);
    font-size: 12px;
    line-height: 20px;

    &__text {
      flex: 1 1 auto;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__link {
      flex: none;
      font-weight: var(--tt-weight-strong);
      white-space: nowrap;
    }

    &__close {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 28px;
      height: 28px;
      border: 0;
      border-radius: var(--tt-radius-control);
      background: transparent;
      color: var(--tt-text-2);
      cursor: pointer;

      &:hover {
        background-color: var(--tt-hover);
      }

      &:focus-visible {
        outline: 2px solid var(--tt-accent-solid);
        outline-offset: 1px;
      }
    }
  }
</style>
