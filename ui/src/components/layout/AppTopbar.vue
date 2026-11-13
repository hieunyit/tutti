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
   * (change password, sign out).
   */
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { CLanguageSelect } from '@clyso/clyso-ui-kit';
  import AppBreadcrumbs from './AppBreadcrumbs.vue';
  import AppQuickSearch from './AppQuickSearch.vue';
  import AppThemeToggle from './AppThemeToggle.vue';
  import HelpMenu from './HelpMenu.vue';
  import i18nLayout from './i18nLayout';
  import AuthUserMenu from '@/components/auth/AuthUserMenu.vue';
  import { useAuthStore } from '@/stores/authStore';
  import { useI18nStore } from '@/stores/i18nStore';
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
  const { locale } = storeToRefs(useI18nStore());
  const { setLocale } = useI18nStore();
  const { showUserMenu } = storeToRefs(useAuthStore());
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

    <AppQuickSearch class="app-topbar__search" />

    <span
      v-if="ENV_LABEL"
      class="app-topbar__env"
      >{{ ENV_LABEL }}</span
    >

    <div class="app-topbar__controls">
      <CLanguageSelect
        v-if="HAS_LANG_SELECT"
        :value="locale"
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
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--tt-space-2) var(--tt-space-4);
    min-height: var(--tt-topbar-height);
    box-sizing: border-box;
    padding: var(--tt-space-2) var(--tt-page-padding);

    @media screen and (max-width: 639px) {
      padding: var(--tt-space-2) var(--tt-space-3);
      gap: var(--tt-space-2);
    }

    &__burger {
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
    }

    &__search {
      margin-left: auto;

      @media screen and (max-width: 639px) {
        order: 10;
        flex: 1 1 100%;
        width: 100%;
        margin-left: 0;
      }
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

      @media screen and (max-width: 639px) {
        margin-left: auto;
      }
    }
  }
</style>
