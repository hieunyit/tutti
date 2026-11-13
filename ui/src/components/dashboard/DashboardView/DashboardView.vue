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
  import { useAuthStore } from '@/stores/authStore';
  import AppSidebar from '@/components/layout/AppSidebar.vue';
  import AppTopbar from '@/components/layout/AppTopbar.vue';
  import Callout from '@/components/ui/Callout.vue';
  import i18nLayout from '@/components/layout/i18nLayout';
  import DashboardFooter from '@/components/dashboard/DashboardFooter/DashboardFooter.vue';
  import { RouteName } from '@/utils/types/router';
  import { useSidebar } from '@/utils/composables/useSidebar';

  const { t } = useI18n({ messages: i18nLayout });
  const route = useRoute();

  const { authEnabled, warnings } = storeToRefs(useAuthStore());

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
    <a
      class="dashboard-view__skip"
      href="#main-content"
      >{{ t('skipToContent') }}</a
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
        <AppTopbar
          :is-mobile="isMobile"
          :drawer-open="isDrawerOpen"
          @open-menu="isDrawerOpen = true"
        />

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

      <main
        id="main-content"
        class="dashboard-view__main"
        tabindex="-1"
      >
        <RouterView />
      </main>

      <DashboardFooter />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .dashboard-view {
    --sidebar-width: 248px;

    &__skip {
      position: fixed;
      top: 8px;
      left: 8px;
      z-index: 20;
      padding: 10px 16px;
      border-radius: var(--tt-radius-control);
      background: var(--tt-accent-solid);
      color: var(--tt-on-accent);
      transform: translateY(-200%);

      &:focus {
        transform: translateY(0);
      }
    }

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

    &__auth-warning {
      margin: 0 var(--tt-page-padding) 8px;

      @media screen and (max-width: 639px) {
        margin: 0 12px 8px;
      }
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
