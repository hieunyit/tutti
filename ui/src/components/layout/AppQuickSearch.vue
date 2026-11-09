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
  import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import i18nLayout from './i18nLayout';
  import { NAV_GROUPS, type NavItem } from '@/utils/constants/navigation';
  import {
    filterSearchItems,
    type SearchableItem,
  } from '@/utils/helpers/SearchHelper';

  const { t } = useI18n({ messages: i18nLayout });
  const { t: tGlobal } = useI18n({ useScope: 'global' });
  const router = useRouter();

  const input = ref<HTMLInputElement | null>(null);
  const query = ref('');
  const isOpen = ref(false);
  const activeIndex = ref(0);

  const entries = computed<SearchableItem<NavItem>[]>(() =>
    NAV_GROUPS.flatMap((group) =>
      group.items.map((item) => ({
        item,
        label: tGlobal(item.routeName),
        group: t(`group.${group.id}`),
      })),
    ),
  );
  const results = computed(() => filterSearchItems(query.value, entries.value));
  const listId = 'quick-search-list';

  function close() {
    isOpen.value = false;
    activeIndex.value = 0;
  }

  function open() {
    isOpen.value = true;
  }

  function go(entry: SearchableItem<NavItem> | undefined) {
    if (!entry) return;

    router.push({ name: entry.item.routeName });
    query.value = '';
    close();
    input.value?.blur();
  }

  function onInput() {
    open();
    activeIndex.value = 0;
  }

  function move(delta: number) {
    open();

    if (results.value.length === 0) return;

    activeIndex.value =
      (activeIndex.value + delta + results.value.length) % results.value.length;
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      move(1);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      move(-1);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      go(results.value[activeIndex.value]);
    } else if (event.key === 'Escape') {
      query.value = '';
      close();
    }
  }

  function onBlur(event: FocusEvent) {
    const next = event.relatedTarget as Node | null;

    if (next && (event.currentTarget as HTMLElement).contains(next)) return;

    close();
  }

  function isTyping(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null;

    if (!el) return false;

    return (
      el.tagName === 'INPUT' ||
      el.tagName === 'TEXTAREA' ||
      el.tagName === 'SELECT' ||
      el.isContentEditable
    );
  }

  function onGlobalKeydown(event: KeyboardEvent) {
    if (
      event.key === '/' &&
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey &&
      !isTyping(event.target)
    ) {
      event.preventDefault();
      nextTick(() => input.value?.focus());
    }
  }

  onMounted(() => document.addEventListener('keydown', onGlobalKeydown));
  onBeforeUnmount(() =>
    document.removeEventListener('keydown', onGlobalKeydown),
  );
</script>

<template>
  <div
    class="quick-search"
    role="search"
    @focusout="onBlur"
  >
    <label class="quick-search__field">
      <svg
        aria-hidden="true"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <circle
          cx="11"
          cy="11"
          r="7"
        />
        <path d="M20 20l-3.5-3.5" />
      </svg>
      <input
        ref="input"
        v-model="query"
        class="quick-search__input"
        type="text"
        role="combobox"
        autocomplete="off"
        aria-autocomplete="list"
        :aria-expanded="isOpen"
        :aria-controls="listId"
        :aria-activedescendant="
          isOpen && results.length > 0 ? `${listId}-${activeIndex}` : undefined
        "
        :aria-label="t('searchLabel')"
        :placeholder="t('searchPlaceholder')"
        @focus="open"
        @input="onInput"
        @keydown="onKeydown"
      />
      <kbd
        class="quick-search__key"
        aria-hidden="true"
        >/</kbd
      >
    </label>

    <ul
      v-show="isOpen"
      :id="listId"
      class="quick-search__list"
      role="listbox"
      :aria-label="t('searchResults')"
    >
      <li
        v-for="(entry, index) in results"
        :id="`${listId}-${index}`"
        :key="entry.item.routeName"
        class="quick-search__option"
        :class="{ 'quick-search__option--active': index === activeIndex }"
        role="option"
        :aria-selected="index === activeIndex"
        @mousedown.prevent
        @click="go(entry)"
        @mousemove="activeIndex = index"
      >
        <span>{{ entry.label }}</span>
        <span class="quick-search__group">{{ entry.group }}</span>
      </li>
      <li
        v-if="results.length === 0"
        class="quick-search__empty"
        role="presentation"
      >
        {{ t('searchNoResults') }}
      </li>
    </ul>
  </div>
</template>

<style lang="scss" scoped>
  .quick-search {
    position: relative;
    width: 300px;
    max-width: 100%;

    &__field {
      display: flex;
      align-items: center;
      gap: 8px;
      box-sizing: border-box;
      height: 36px;
      padding: 0 10px;
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-surface);
      color: var(--tt-text-3);
      font-size: 13px;

      &:focus-within {
        outline: 2px solid var(--tt-accent);
        outline-offset: 0;
      }
    }

    &__input {
      flex: 1;
      min-width: 0;
      border: 0;
      outline: 0;
      background: transparent;
      font: inherit;
      color: var(--tt-text);

      &::placeholder {
        color: var(--tt-text-3);
      }
    }

    &__key {
      padding: 0 5px;
      border: 1px solid var(--tt-border);
      border-radius: 4px;
      font-family: inherit;
      font-size: 11px;
    }

    &__list {
      position: absolute;
      z-index: 20;
      top: calc(100% + 4px);
      left: 0;
      right: 0;
      max-height: 320px;
      margin: 0;
      padding: 4px;
      overflow-y: auto;
      list-style: none;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      box-shadow: 0 8px 24px rgba(16, 24, 40, 0.18);
    }

    &__option {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 8px 10px;
      border-radius: 6px;
      font-size: 13px;
      color: var(--tt-text);
      cursor: pointer;

      &--active {
        background-color: var(--tt-accent-subtle);
      }
    }

    &__group {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__empty {
      padding: 8px 10px;
      font-size: 13px;
      color: var(--tt-text-3);
    }
  }
</style>
