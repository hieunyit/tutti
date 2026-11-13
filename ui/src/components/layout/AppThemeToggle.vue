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
   * Light / dark / system theme menu. "System" is the absence of a saved
   * choice: the store then follows prefers-color-scheme (also live, here).
   */
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NDropdown, type DropdownOption } from 'naive-ui';
  import { ColorScheme } from '@clyso/clyso-ui-kit';
  import i18nLayout from './i18nLayout';
  import { useColorSchemeStore } from '@/stores/colorSchemeStore';
  import LocalStorageHelper from '@/utils/helpers/LocalStorageHelper';
  import { LocalStorageItem } from '@/utils/types/localStorage';

  type ThemeMode = 'light' | 'dark' | 'system';

  const { t } = useI18n({ messages: i18nLayout });
  const store = useColorSchemeStore();
  const { isDark } = storeToRefs(store);
  const media =
    typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-color-scheme: dark)')
      : null;

  const mode = ref<ThemeMode>(
    LocalStorageHelper.get(LocalStorageItem.COLOR_SCHEME) === null
      ? 'system'
      : isDark.value
        ? 'dark'
        : 'light',
  );

  const label = computed(
    () =>
      `${t('themeLabel')}: ${
        mode.value === 'system'
          ? t('themeOptionSystem')
          : mode.value === 'dark'
            ? t('themeOptionDark')
            : t('themeOptionLight')
      }`,
  );

  const options = computed<DropdownOption[]>(() =>
    (['light', 'dark', 'system'] as const).map((key) => ({
      key,
      label:
        (key === mode.value ? '✓ ' : '') +
        (key === 'light'
          ? t('themeOptionLight')
          : key === 'dark'
            ? t('themeOptionDark')
            : t('themeOptionSystem')),
    })),
  );

  function followSystem() {
    store.setColorScheme(media?.matches ? ColorScheme.DARK : ColorScheme.LIGHT);
    LocalStorageHelper.remove(LocalStorageItem.COLOR_SCHEME);
  }

  function select(key: string | number) {
    mode.value = key as ThemeMode;

    if (key === 'system') {
      followSystem();

      return;
    }

    store.setColorScheme(key === 'dark' ? ColorScheme.DARK : ColorScheme.LIGHT);
  }

  function onSystemChange() {
    if (mode.value === 'system') followSystem();
  }

  onMounted(() => media?.addEventListener('change', onSystemChange));
  onBeforeUnmount(() => media?.removeEventListener('change', onSystemChange));
</script>

<template>
  <NDropdown
    trigger="click"
    :options="options"
    @select="select"
  >
    <button
      type="button"
      class="theme-toggle"
      :aria-label="label"
      :title="label"
      aria-haspopup="menu"
    >
      <svg
        v-if="isDark"
        aria-hidden="true"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
      </svg>
      <svg
        v-else
        aria-hidden="true"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
      >
        <circle
          cx="12"
          cy="12"
          r="4"
        />
        <path
          d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"
        />
      </svg>
    </button>
  </NDropdown>
</template>

<style lang="scss" scoped>
  .theme-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    width: 36px;
    height: 36px;
    border: 1px solid var(--tt-border-strong);
    border-radius: var(--tt-radius-control);
    background-color: var(--tt-surface);
    color: var(--tt-text-2);
    cursor: pointer;

    &:hover {
      background-color: var(--tt-subtle);
      color: var(--tt-text);
    }
  }
</style>
