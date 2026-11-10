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
  import { CLanguageSelect } from '@clyso/clyso-ui-kit';
  import { useI18nStore } from '@/stores/i18nStore';
  import AuthUserMenu from '@/components/auth/AuthUserMenu.vue';
  import { useAuthStore } from '@/stores/authStore';
  import AppSidebar from '@/components/layout/AppSidebar.vue';
  import AppBreadcrumbs from '@/components/layout/AppBreadcrumbs.vue';
  import AppQuickSearch from '@/components/layout/AppQuickSearch.vue';
  import AppThemeToggle from '@/components/layout/AppThemeToggle.vue';
  import Callout from '@/components/ui/Callout.vue';
  import i18nLayout from '@/components/layout/i18nLayout';
  import DashboardFooter from '@/components/dashboard/DashboardFooter/DashboardFooter.vue';
  import { ENV_LABEL, HAS_LANG_SELECT } from '@/utils/constants/env';
  import { RouteName } from '@/utils/types/router';
  import { useSidebar } from '@/utils/composables/useSidebar';

  const { t } = useI18n({ messages: i18nLayout });
  const route = useRoute();

  const { locale } = storeToRefs(useI18nStore());
  const { setLocale } = useI18nStore();

  const { showUserMenu, authEnabled, warnings } = storeToRefs(useAuthStore());

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
      <div class="dashboard-view__top">
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

          <AppQuickSearch class="dashboard-view__search" />

          <span
            v-if="ENV_LABEL"
            class="dashboard-view__env"
            >{{ ENV_LABEL }}</span
          >

          <div class="dashboard-view__controls">
            <CLanguageSelect
              v-if="HAS_LANG_SELECT"
              :value="locale"
              @update:value="setLocale"
            />

            <AppThemeToggle />

            <AuthUserMenu v-if="showUserMenu" />
          </div>
        </header>

        <Callout
          v-if="authEnabled === false"
          class="dashboard-view__auth-warning"
          tone="warning"
        >
          {{ t('authDisabledBanner') }}
          <template #actions>
            <RouterLink :to="{ name: RouteName.TUTTI_DOCS }">{{
              t('authDisabledLink')
            }}</RouterLink>
          </template>
        </Callout>
        <Callout
          v-if="warnings.includes('webhook_unauthenticated')"
          class="dashboard-view__auth-warning"
          tone="warning"
        >
          {{ t('webhookUnauthenticated') }}
          <template #actions>
            <RouterLink
              :to="{
                name: RouteName.TUTTI_DOCS,
                params: { slug: 'configuration' },
              }"
              >{{ t('authDisabledLink') }}</RouterLink
            >
          </template>
        </Callout>
      </div>

      <main class="dashboard-view__main">
        <RouterView />
      </main>

      <DashboardFooter />
    </div>
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

    &__top {
      position: sticky;
      top: 0;
      z-index: 5;
      background-color: var(--tt-surface);
      border-bottom: 1px solid var(--tt-border);
    }

    &__topbar {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px 16px;
      min-height: 56px;
      box-sizing: border-box;
      padding: 8px var(--tt-page-padding);

      @media screen and (max-width: 639px) {
        padding: 8px 12px;
        gap: 8px;
      }
    }

    &__auth-warning {
      margin: 0 var(--tt-page-padding) 8px;

      @media screen and (max-width: 639px) {
        margin: 0 12px 8px;
      }
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
      font-size: 12px;
      line-height: 16px;
      font-weight: 600;
      color: var(--tt-warning);
    }

    &__burger {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 36px;
      height: 36px;
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

    &__controls {
      display: flex;
      align-items: center;
      flex: none;
      gap: 8px;
    }

    &__main {
      flex: 1;
      min-width: 0;
      padding: var(--tt-page-padding);

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
