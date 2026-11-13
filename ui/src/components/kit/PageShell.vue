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
   * Frame of every page: title, description, primary actions on the right,
   * optional tabs (bind activeTab to useQueryTab to keep them in the URL) and
   * the content, which follows `state`: loading skeleton, empty slot,
   * ErrorBlock with retry, or the default slot.
   */
  import { nextTick, ref, useId } from 'vue';
  import { EmptyBlock, ErrorBlock, LoadingBlock } from './stateBlocks';
  import type { PageShellTab } from './types';

  const props = withDefaults(
    defineProps<{
      title: string;
      description?: string;
      state: 'loading' | 'empty' | 'error' | 'ready';
      tabs?: PageShellTab[];
      activeTab?: string;
      /** optional text under the error title (e.g. the API message) */
      errorText?: string;
    }>(),
    {
      description: undefined,
      tabs: undefined,
      activeTab: undefined,
      errorText: undefined,
    },
  );

  const emit = defineEmits<{
    (e: 'update:activeTab', key: string): void;
    (e: 'retry'): void;
  }>();

  const tabList = ref<HTMLElement | null>(null);
  const uid = `kit-page-${useId()}`;

  function select(key: string) {
    if (key !== props.activeTab) emit('update:activeTab', key);
  }

  function onTabKeydown(event: KeyboardEvent, index: number) {
    const tabs = props.tabs ?? [];
    let next = -1;

    if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
    else if (event.key === 'ArrowLeft')
      next = (index - 1 + tabs.length) % tabs.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = tabs.length - 1;

    const target = tabs[next];

    if (!target) return;

    event.preventDefault();
    select(target.key);
    nextTick(() =>
      tabList.value
        ?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
        [next]?.focus(),
    );
  }
</script>

<template>
  <section class="kit-page">
    <header class="kit-page__header">
      <div
        v-if="$slots.back"
        class="kit-page__back"
      >
        <slot name="back" />
      </div>
      <div class="kit-page__row">
        <div class="kit-page__heading">
          <div class="kit-page__title-row">
            <h1 class="kit-page__title">{{ title }}</h1>
            <slot name="meta" />
          </div>
          <p
            v-if="description"
            class="kit-page__description"
          >
            {{ description }}
          </p>
        </div>
        <div
          v-if="$slots.actions"
          class="kit-page__actions"
        >
          <slot name="actions" />
        </div>
      </div>

      <div
        v-if="tabs && tabs.length > 0"
        ref="tabList"
        class="kit-page__tabs"
        role="tablist"
      >
        <button
          v-for="(tab, index) in tabs"
          :id="`${uid}-tab-${tab.key}`"
          :key="tab.key"
          type="button"
          role="tab"
          class="kit-page__tab"
          :class="{ 'kit-page__tab--active': tab.key === activeTab }"
          :aria-selected="tab.key === activeTab ? 'true' : 'false'"
          :aria-controls="`${uid}-panel`"
          :tabindex="tab.key === activeTab ? 0 : -1"
          @click="select(tab.key)"
          @keydown="onTabKeydown($event, index)"
        >
          {{ tab.label }}
          <span
            v-if="tab.badge"
            class="kit-page__badge"
            >{{ tab.badge }}</span
          >
        </button>
      </div>
    </header>

    <div
      :id="`${uid}-panel`"
      class="kit-page__body"
      :role="tabs && tabs.length > 0 ? 'tabpanel' : undefined"
      :aria-labelledby="
        tabs && tabs.length > 0 && activeTab
          ? `${uid}-tab-${activeTab}`
          : undefined
      "
    >
      <LoadingBlock
        v-if="state === 'loading'"
        :rows="6"
      />
      <ErrorBlock
        v-else-if="state === 'error'"
        :description="errorText"
        @retry="emit('retry')"
      />
      <template v-else-if="state === 'empty'">
        <slot name="empty">
          <EmptyBlock />
        </slot>
      </template>
      <slot v-else />
    </div>
  </section>
</template>

<style lang="scss" scoped>
  .kit-page {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);
    width: 100%;
    max-width: var(--tt-content-max);
    margin: 0 auto;

    &__header {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
    }

    &__row {
      display: flex;
      align-items: flex-end;
      flex-wrap: wrap;
      gap: var(--tt-space-3) var(--tt-space-4);
    }

    &__heading {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      flex: 1 1 320px;
      min-width: 0;
    }

    &__title-row {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2) var(--tt-space-3);
      min-width: 0;
    }

    &__title {
      margin: 0;
      font-size: var(--tt-text-2xl);
      line-height: 32px;
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text);
      overflow-wrap: anywhere;
    }

    &__description {
      margin: 0;
      max-width: var(--tt-measure);
      font-size: var(--tt-text-md);
      line-height: 20px;
      color: var(--tt-text-2);
    }

    &__actions {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      margin-left: auto;
    }

    &__tabs {
      display: flex;
      gap: var(--tt-space-1);
      overflow-x: auto;
      border-bottom: 1px solid var(--tt-border);
      scrollbar-width: thin;
    }

    &__tab {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      flex: none;
      min-height: 40px;
      padding: 0 var(--tt-space-3);
      border: 0;
      border-bottom: 2px solid transparent;
      margin-bottom: -1px;
      background: transparent;
      font: inherit;
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text-2);
      cursor: pointer;
      transition: color var(--tt-duration-fast) var(--tt-ease);

      &:hover {
        color: var(--tt-text);
      }

      &:focus-visible {
        outline-offset: -2px;
      }

      &--active {
        color: var(--tt-accent);
        border-bottom-color: var(--tt-accent);
      }
    }

    &__badge {
      min-width: 20px;
      padding: 0 6px;
      border-radius: var(--tt-radius-pill);
      font-size: var(--tt-text-xs);
      line-height: 18px;
      text-align: center;
      color: var(--tt-warning);
      background-color: var(--tt-warning-bg);
      border: 1px solid var(--tt-warning-border);
    }

    &__body {
      min-width: 0;
    }
  }
</style>
