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
   * Sidebar shell: brand, the data-driven navigation (utils/constants/navigation.ts,
   * filtered by role) and the footer. `collapsed` is the icon-only rail
   * (desktop); `isDrawer` renders it inside the mobile drawer (<= 1024 px).
   */
  import { computed } from 'vue';
  import { useRoute } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import i18nLayout from './i18nLayout';
  import SidebarFooter from './sidebar/SidebarFooter.vue';
  import SidebarNavGroup from './sidebar/SidebarNavGroup.vue';
  import { visibleNavGroups } from '@/utils/constants/navigation';
  import { useAuthStore } from '@/stores/authStore';
  import { useNavBadges } from '@/utils/composables/useNavBadges';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    /** icon-only rail (desktop only) */
    collapsed?: boolean;
    /** rendered inside the mobile drawer */
    isDrawer?: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'navigate'): void;
    (e: 'toggle'): void;
  }>();

  const { t } = useI18n({ messages: i18nLayout });
  const { t: tRoute } = useI18n({ useScope: 'global' });
  const route = useRoute();
  const badges = useNavBadges();
  const { isAdmin } = storeToRefs(useAuthStore());

  // Viewers do not see admin-only items (Console users, System settings,
  // Audit log); the router guard blocks the URLs as well.
  const groups = computed(() => visibleNavGroups(isAdmin.value));
  const isRail = computed(() => Boolean(props.collapsed) && !props.isDrawer);
  const activeItem = computed<string | undefined>(() =>
    String(route.meta.navSection ?? route.name ?? ''),
  );
  const NAV_ID = 'app-sidebar-nav';
</script>

<template>
  <aside
    class="app-sidebar"
    :class="{ 'app-sidebar--rail': isRail }"
  >
    <RouterLink
      class="app-sidebar__brand"
      :to="{ name: RouteName.TUTTI_DASHBOARD }"
      aria-label="Tutti"
      @click="emit('navigate')"
    >
      <span
        class="app-sidebar__logo"
        aria-hidden="true"
        >T</span
      >
      <span
        v-if="!isRail"
        class="app-sidebar__brand-text"
      >
        <span class="app-sidebar__product">Tutti</span>
        <span class="app-sidebar__tagline">{{ t('consoleTagline') }}</span>
      </span>
    </RouterLink>

    <nav
      :id="NAV_ID"
      class="app-sidebar__nav"
      :aria-label="t('navLabel')"
    >
      <SidebarNavGroup
        v-for="group in groups"
        :key="group.id"
        :group="group"
        :label="t(`group.${group.id}`)"
        :rail="isRail"
        :active-item="activeItem"
        :badges="badges"
        :item-label="(name) => tRoute(name)"
        @navigate="emit('navigate')"
      />
    </nav>

    <SidebarFooter
      :collapsed="Boolean(collapsed)"
      :is-drawer="Boolean(isDrawer)"
      :rail="isRail"
      :controls="NAV_ID"
      @toggle="emit('toggle')"
    />
  </aside>
</template>

<style lang="scss" scoped>
  .app-sidebar {
    display: flex;
    flex-direction: column;
    gap: 4px;
    height: 100%;
    min-height: 0;
    width: 100%;
    box-sizing: border-box;
    background-color: var(--tt-nav-bg);
    color: var(--tt-nav-text);
    border-right: 1px solid var(--tt-nav-border);
    padding: 16px 12px;
    overflow: hidden;

    &__brand {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 4px 8px 16px;
      color: var(--tt-nav-strong);
      text-decoration: none;
      border-radius: var(--tt-radius-control);

      &:focus-visible {
        outline: 2px solid var(--tt-nav-accent);
        outline-offset: -2px;
      }
    }

    &__logo {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 32px;
      height: 32px;
      border-radius: 8px;
      font-weight: 700;
      font-size: 16px;
      color: var(--tt-nav-strong);
      background-color: var(--tt-nav-logo);
    }

    &__brand-text {
      display: flex;
      flex-direction: column;
      min-width: 0;
    }

    &__product {
      font-size: 15px;
      line-height: 20px;
      font-weight: 600;
      color: var(--tt-nav-strong);
    }

    &__tagline {
      font-size: 12px;
      line-height: 16px;
      color: var(--tt-nav-muted);
      white-space: nowrap;
    }

    &__nav {
      flex: 1;
      min-height: 0;
      overflow-y: auto;
    }

    &--rail {
      padding-inline: 6px;

      .app-sidebar__brand {
        justify-content: center;
        padding-inline: 0;
      }
    }
  }
</style>
