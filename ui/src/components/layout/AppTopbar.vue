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
   * Top bar: menu button (mobile), breadcrumbs, quick search (Ctrl/Cmd+K),
   * environment label, language, Help menu, theme menu and the user menu
   * (change password, sign out). On phones (<= 640px) it is a single row:
   * menu button, page title, overflow menu (help, theme, language) and the
   * user menu; the quick search is not rendered (the drawer is the navigation).
   */
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NSelect } from 'naive-ui';
  import AppBreadcrumbs from './AppBreadcrumbs.vue';
  import AppQuickSearch from './AppQuickSearch.vue';
  import AppThemeToggle from './AppThemeToggle.vue';
  import HelpMenu from './HelpMenu.vue';
  import TopbarOverflow from './TopbarOverflow.vue';
  import i18nLayout from './i18nLayout';
  import AuthUserMenu from '@/components/auth/AuthUserMenu.vue';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useAuthStore } from '@/stores/authStore';
  import { useI18nStore } from '@/stores/i18nStore';
  import { APP_LOCALE_LABELS } from '@/i18n/locales';
  import { ENV_LABEL, HAS_LANG_SELECT } from '@/utils/constants/env';

  defineProps<{
    /** narrow screens: show the menu button that opens the drawer */
    isMobile?: boolean;
    drawerOpen?: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'open-menu'): void;
  }>();

  const { t } = useI18n({ messages: i18nLayout });
  const { locale, localeList } = storeToRefs(useI18nStore());
  const localeOptions = computed(() =>
    localeList.value.map((value) => ({
      value,
      label: APP_LOCALE_LABELS[value],
    })),
  );
  const { setLocale } = useI18nStore();
  const { showUserMenu } = storeToRefs(useAuthStore());
  // phones (<= 640px): one 56px row, the rest lives in the overflow menu
  const isPhone = useNarrowScreen(641);
</script>

<template>
  <header
    class="app-topbar"
    :aria-label="t('topbarLabel')"
  >
    <button
      v-if="isMobile"
      type="button"
      class="app-topbar__burger"
      :aria-label="t('openMenu')"
      :aria-expanded="drawerOpen"
      @click="emit('open-menu')"
    >
      <svg
        width="20"
        height="20"
        viewBox="0 0 20 20"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        aria-hidden="true"
      >
        <path d="M3 5h14M3 10h14M3 15h14" />
      </svg>
    </button>

    <AppBreadcrumbs class="app-topbar__crumbs" />

    <AppQuickSearch
      v-if="!isPhone"
      class="app-topbar__search"
    />

    <span
      v-if="ENV_LABEL && !isPhone"
      class="app-topbar__env"
      >{{ ENV_LABEL }}</span
    >

    <div
      v-if="isPhone"
      class="app-topbar__controls"
    >
      <TopbarOverflow />
      <AuthUserMenu v-if="showUserMenu" />
    </div>

    <div
      v-else
      class="app-topbar__controls"
    >
      <NSelect
        v-if="HAS_LANG_SELECT"
        class="app-topbar__lang"
        size="small"
        :value="locale"
        :options="localeOptions"
        :consistent-menu-width="false"
        :aria-label="t('languageLabel')"
        @update:value="setLocale"
      />
      <HelpMenu />
      <AppThemeToggle />
      <AuthUserMenu v-if="showUserMenu" />
    </div>
  </header>
</template>

<style lang="scss" scoped>
  .app-topbar {
    &__lang {
      width: 120px;
    }

    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--tt-space-2) var(--tt-space-4);
    min-height: var(--tt-topbar-height);
    box-sizing: border-box;
    padding: var(--tt-space-2) var(--tt-page-padding);

    @media screen and (max-width: 640px) {
      flex-wrap: nowrap;
      height: 56px;
      min-height: 56px;
      padding: 0 var(--tt-space-2);
      gap: var(--tt-space-2);
    }

    &__burger {
      &:focus-visible {
        outline: 2px solid var(--tt-accent-solid);
        outline-offset: 2px;
      }

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
    }

    &__crumbs {
      flex: 0 1 auto;
      min-width: 0;

      @media screen and (max-width: 640px) {
        flex: 1 1 auto;
      }
    }

    &__search {
      margin-left: auto;
    }

    &__env {
      padding: 2px 10px;
      border: 1px solid var(--tt-warning-border);
      border-radius: var(--tt-radius-pill);
      background-color: var(--tt-warning-bg);
      font-size: var(--tt-text-xs);
      line-height: 16px;
      font-weight: var(--tt-weight-strong);
      color: var(--tt-warning);
    }

    &__controls {
      display: flex;
      align-items: center;
      flex: none;
      gap: var(--tt-space-2);

      @media screen and (max-width: 640px) {
        gap: 0;
      }
    }
  }
</style>
