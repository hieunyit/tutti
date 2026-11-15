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
   * Phone top bar overflow menu: help, theme and language in one dropdown
   * (the user menu stays next to it as its own avatar button).
   */
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NDropdown, type DropdownOption } from 'naive-ui';
  import i18nLayout from './i18nLayout';
  import { useThemeMode } from './useThemeMode';
  import { useI18nStore } from '@/stores/i18nStore';
  import { APP_LOCALE_LABELS } from '@/i18n/locales';
  import { HAS_LANG_SELECT } from '@/utils/constants/env';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nLayout });
  const router = useRouter();
  const { mode, select: selectTheme } = useThemeMode();
  const { locale, localeList } = storeToRefs(useI18nStore());
  const { setLocale } = useI18nStore();

  const mark = (active: boolean) => (active ? '✓ ' : '');

  const options = computed<DropdownOption[]>(() => {
    const list: DropdownOption[] = [
      {
        type: 'group',
        key: 'help',
        label: t('help'),
        children: [
          { key: 'help:docs', label: t('helpDocs') },
          { key: 'help:api', label: t('helpApi') },
        ],
      },
      {
        type: 'group',
        key: 'theme',
        label: t('themeLabel'),
        children: [
          ['light', t('themeOptionLight')],
          ['dark', t('themeOptionDark')],
          ['system', t('themeOptionSystem')],
        ].map(([key, label]) => ({
          key: `theme:${key}`,
          label: mark(mode.value === key) + label,
        })),
      },
    ];

    if (HAS_LANG_SELECT) {
      list.push({
        type: 'group',
        key: 'lang',
        label: t('languageLabel'),
        children: localeList.value.map((value) => ({
          key: `lang:${value}`,
          label: mark(locale.value === value) + APP_LOCALE_LABELS[value],
        })),
      });
    }

    return list;
  });

  function onSelect(key: string | number) {
    const [kind, value = ''] = String(key).split(':');

    if (kind === 'help') {
      router.push({
        name:
          value === 'api'
            ? RouteName.TUTTI_API_REFERENCE
            : RouteName.TUTTI_DOCS,
      });
    } else if (kind === 'theme') {
      selectTheme(value);
    } else if (kind === 'lang') {
      setLocale(value as typeof locale.value);
    }
  }
</script>

<template>
  <NDropdown
    trigger="click"
    :options="options"
    @select="onSelect"
  >
    <button
      type="button"
      class="topbar-overflow"
      :aria-label="t('moreOptions')"
      :title="t('moreOptions')"
      aria-haspopup="menu"
    >
      <svg
        aria-hidden="true"
        width="20"
        height="20"
        viewBox="0 0 20 20"
        fill="currentColor"
      >
        <circle
          cx="10"
          cy="4"
          r="1.6"
        />
        <circle
          cx="10"
          cy="10"
          r="1.6"
        />
        <circle
          cx="10"
          cy="16"
          r="1.6"
        />
      </svg>
    </button>
  </NDropdown>
</template>

<style lang="scss" scoped>
  .topbar-overflow {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    width: 40px;
    height: 40px;
    border: 0;
    border-radius: var(--tt-radius-control);
    background: transparent;
    color: var(--tt-text);
    cursor: pointer;

    &:hover {
      background-color: var(--tt-hover);
    }

    &:focus-visible {
      outline: 2px solid var(--tt-accent-solid);
      outline-offset: 2px;
    }
  }
</style>
