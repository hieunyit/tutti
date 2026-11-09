<!--
  - Copyright © 2026 Clyso GmbH
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
  import { storeToRefs } from 'pinia';
  import { watch } from 'vue';
  import { useRoute } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NDrawer, NDrawerContent } from 'naive-ui';
  import { CColorSchemeToggle, CLanguageSelect } from '@clyso/clyso-ui-kit';
  import { useI18nStore } from '@/stores/i18nStore';
  import { useColorSchemeStore } from '@/stores/colorSchemeStore';
  import AuthUserMenu from '@/components/auth/AuthUserMenu.vue';
  import { useAuthStore } from '@/stores/authStore';
  import AppSidebar from '@/components/layout/AppSidebar.vue';
  import AppBreadcrumbs from '@/components/layout/AppBreadcrumbs.vue';
  import AboutDialog from '@/components/layout/AboutDialog.vue';
  import i18nLayout from '@/components/layout/i18nLayout';
  import DashboardFooter from '@/components/dashboard/DashboardFooter/DashboardFooter.vue';
  import { HAS_LANG_SELECT } from '@/utils/constants/env';
  import { useSidebar } from '@/utils/composables/useSidebar';

  const { t } = useI18n({ messages: i18nLayout });
  const route = useRoute();

  const { locale } = storeToRefs(useI18nStore());
  const { setLocale } = useI18nStore();

  const { showUserMenu } = storeToRefs(useAuthStore());

  const { colorScheme } = storeToRefs(useColorSchemeStore());
  const { setColorScheme } = useColorSchemeStore();

  const { isCollapsed, isMobile, isDrawerOpen, toggleCollapsed } = useSidebar();

  watch(
    () => route.fullPath,
    () => {
      isDrawerOpen.value = false;
    },
  );
</script>

<template>
  <div
    class="dashboard-view"
    :class="{
      'dashboard-view--collapsed': isCollapsed && !isMobile,
      'dashboard-view--mobile': isMobile,
    }"
  >
    <div
      v-if="!isMobile"
      class="dashboard-view__sidebar"
    >
      <AppSidebar
        :collapsed="isCollapsed"
        @toggle="toggleCollapsed"
      />
    </div>

    <NDrawer
      v-else
      v-model:show="isDrawerOpen"
      placement="left"
      :width="264"
    >
      <NDrawerContent
        :native-scrollbar="false"
        :body-content-style="{ padding: 0, height: '100%' }"
        class="dashboard-view__drawer-content"
      >
        <AppSidebar
          is-drawer
          @navigate="isDrawerOpen = false"
        />
      </NDrawerContent>
    </NDrawer>

    <div class="dashboard-view__body">
      <header class="dashboard-view__topbar">
        <button
          v-if="isMobile"
          type="button"
          class="dashboard-view__burger"
          :aria-label="t('openMenu')"
          :aria-expanded="isDrawerOpen"
          @click="isDrawerOpen = true"
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

        <AppBreadcrumbs class="dashboard-view__crumbs" />

        <div class="dashboard-view__controls">
          <CLanguageSelect
            v-if="HAS_LANG_SELECT"
            :value="locale"
            @update:value="setLocale"
          />

          <CColorSchemeToggle
            :value="colorScheme"
            @update:value="setColorScheme"
          />

          <AuthUserMenu v-if="showUserMenu" />
        </div>
      </header>

      <main class="dashboard-view__main">
        <RouterView />
      </main>

      <DashboardFooter />
    </div>

    <AboutDialog />
  </div>
</template>

<style lang="scss" scoped>
  .dashboard-view {
    --sidebar-width: 248px;

    display: grid;
    grid-template-columns: var(--sidebar-width) minmax(0, 1fr);
    min-height: 100vh;
    max-width: 100vw;

    &--collapsed {
      --sidebar-width: 64px;
    }

    &--mobile {
      grid-template-columns: minmax(0, 1fr);
    }

    &__sidebar {
      position: sticky;
      top: 0;
      height: 100vh;
      z-index: 3;
    }

    &__body {
      display: flex;
      flex-direction: column;
      min-width: 0;
      min-height: 100vh;
    }

    &__topbar {
      position: sticky;
      top: 0;
      z-index: 2;
      display: flex;
      align-items: center;
      gap: 12px;
      height: 56px;
      padding: 0 24px;
      background-color: var(--card-color);
      border-bottom: 1px solid var(--border-color);

      @media screen and (max-width: 639px) {
        padding: 0 12px;
        gap: 8px;
      }
    }

    &__burger {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 36px;
      height: 36px;
      border: 0;
      border-radius: 6px;
      background: transparent;
      color: var(--text-color-1);
      cursor: pointer;

      &:hover {
        background-color: var(--button-color-2-hover);
      }

      &:focus-visible {
        outline: 2px solid var(--primary-color);
      }
    }

    &__crumbs {
      flex: 1;
      min-width: 0;
    }

    &__controls {
      display: flex;
      align-items: center;
      flex: none;
      gap: 8px;
    }

    &__main {
      flex: 1;
      min-width: 0;
      padding: 24px;

      @media screen and (max-width: 639px) {
        padding: 16px;

        // interim until the per-page pass: wide tables scroll inside their
        // card instead of crushing their columns or scrolling the page
        :deep(.c-data-table-base-table) {
          overflow-x: auto;
        }

        :deep(.c-data-table-table) {
          min-width: 680px;
        }
      }
    }
  }
</style>
