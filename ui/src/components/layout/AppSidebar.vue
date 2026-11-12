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
  import { computed } from 'vue';
  import { useRoute } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { NTooltip } from 'naive-ui';
  import i18nLayout from './i18nLayout';
  import { i18n } from '@/i18n';
  import { NAV_GROUPS, type NavItem } from '@/utils/constants/navigation';
  import { useAuthStore } from '@/stores/authStore';
  import { useNavBadges } from '@/utils/composables/useNavBadges';
  import { APP_VERSION } from '@/utils/constants/env';
  import { AUDIT_LOG_ROUTE_NAME } from '@/utils/types/audit';
  import { RouteName } from '@/utils/types/router';

  i18n.global.mergeLocaleMessage('en', {
    [AUDIT_LOG_ROUTE_NAME]: 'Audit log',
  });
  i18n.global.mergeLocaleMessage('de', {
    [AUDIT_LOG_ROUTE_NAME]: 'Audit-Protokoll',
  });
  i18n.global.mergeLocaleMessage('vi', {
    [AUDIT_LOG_ROUTE_NAME]: 'Nhật ký kiểm toán',
    [RouteName.TUTTI_SYSTEM_SETTINGS]: 'Cấu hình hệ thống',
  });

  const AUDIT_NAV_ITEM: NavItem = {
    routeName: AUDIT_LOG_ROUTE_NAME as unknown as RouteName,
    iconPath:
      'M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2m-6 7h6m-6 4h4',
  };

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
  const route = useRoute();
  const badges = useNavBadges();
  const { isAdmin } = storeToRefs(useAuthStore());
  const groups = computed(() =>
    NAV_GROUPS.map((group) => {
      const filteredItems = group.items.filter(
        (item) => !item.adminOnly || isAdmin.value,
      );

      if (group.id !== 'administration') {
        return !group.adminOnly || isAdmin.value
          ? { ...group, items: filteredItems }
          : { ...group, items: [] };
      }

      const baseItems = isAdmin.value ? filteredItems : [];
      const hasAudit = baseItems.some(
        (item) => String(item.routeName) === AUDIT_LOG_ROUTE_NAME,
      );

      return {
        ...group,
        // the audit log is admin-only on the server (ListAuditLog), so viewers must not see it
        items:
          hasAudit || !isAdmin.value
            ? baseItems
            : [...baseItems, AUDIT_NAV_ITEM],
      };
    }).filter((group) => group.items.length > 0),
  );

  const isRail = computed(() => Boolean(props.collapsed) && !props.isDrawer);

  const activeItem = computed<string | undefined>(() =>
    String(route.meta.navSection ?? route.name ?? ''),
  );
</script>

<template>
  <aside
    class="app-sidebar"
    :class="{ 'app-sidebar--rail': isRail }"
  >
    <RouterLink
      class="app-sidebar__brand"
      :to="{ name: RouteName.CHORUS_HOME }"
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
      class="app-sidebar__nav"
      :aria-label="t('navLabel')"
    >
      <section
        v-for="group in groups"
        :key="group.id"
        class="app-sidebar__group"
      >
        <h2
          v-if="!isRail"
          class="app-sidebar__group-label"
        >
          {{ t(`group.${group.id}`) }}
        </h2>
        <div
          v-else
          class="app-sidebar__group-divider"
        />

        <NTooltip
          v-for="item in group.items"
          :key="item.routeName"
          placement="right"
          :disabled="!isRail"
        >
          <template #trigger>
            <RouterLink
              class="app-sidebar__item"
              :class="{
                'app-sidebar__item--active': activeItem === item.routeName,
              }"
              :to="{ name: item.routeName }"
              :aria-label="$t(item.routeName)"
              :aria-current="activeItem === item.routeName ? 'page' : undefined"
              @click="emit('navigate')"
            >
              <svg
                class="app-sidebar__icon"
                aria-hidden="true"
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path :d="item.iconPath" />
              </svg>
              <span
                v-if="!isRail"
                class="app-sidebar__label"
                >{{ $t(item.routeName) }}</span
              >
              <template v-if="item.badgeKey && badges[item.badgeKey]">
                <span
                  v-if="!isRail"
                  class="app-sidebar__badge"
                >
                  {{ badges[item.badgeKey] }}
                  <span class="tt-sr-only">{{ t('badgeLabel') }}</span>
                </span>
                <span
                  v-else
                  class="app-sidebar__badge-dot"
                  aria-hidden="true"
                />
              </template>
            </RouterLink>
          </template>
          {{ $t(item.routeName) }}
        </NTooltip>
      </section>
    </nav>

    <div class="app-sidebar__footer">
      <div class="app-sidebar__meta">
        <span
          class="app-sidebar__version"
          :title="`Tutti v${APP_VERSION}`"
          >v{{ APP_VERSION }}</span
        >

        <NTooltip
          v-if="!isDrawer"
          placement="right"
        >
          <template #trigger>
            <button
              type="button"
              class="app-sidebar__toggle"
              :aria-label="collapsed ? t('expand') : t('collapse')"
              :aria-expanded="!collapsed"
              @click="emit('toggle')"
            >
              <span aria-hidden="true">{{ collapsed ? '›' : '‹' }}</span>
            </button>
          </template>
          {{ collapsed ? t('expand') : t('collapse') }}
        </NTooltip>
      </div>
    </div>
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
      color: #ffffff;
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

    &__group {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding-top: 12px;
    }

    &__group-label {
      margin: 0;
      padding: 0 10px 6px;
      font-size: 11px;
      line-height: 16px;
      font-weight: 600;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--tt-nav-muted);
    }

    &__group-divider {
      height: 1px;
      margin: 6px 8px;
      background-color: var(--tt-nav-border);
    }

    &__item {
      position: relative;
      display: flex;
      align-items: center;
      gap: 10px;
      box-sizing: border-box;
      width: 100%;
      padding: 8px 10px;
      border-radius: var(--tt-radius-control);
      font-size: 14px;
      line-height: 20px;
      font-weight: 500;
      color: var(--tt-nav-text);
      text-decoration: none;
      transition: background-color 0.15s;

      &:hover {
        background-color: var(--tt-nav-hover-bg);
        color: var(--tt-nav-strong);
      }

      &:focus-visible {
        outline: 2px solid var(--tt-nav-accent);
        outline-offset: -2px;
      }

      &--active {
        background-color: var(--tt-nav-active-bg);
        color: #ffffff;
        font-weight: 600;
        box-shadow: inset 2px 0 0 var(--tt-nav-accent);
      }
    }

    &__icon {
      flex: none;
    }

    &__label {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__badge {
      margin-left: auto;
      padding: 1px 7px;
      border-radius: var(--tt-radius-pill);
      font-size: 11px;
      line-height: 16px;
      font-weight: 600;
      color: var(--tt-nav-badge-text);
      background-color: var(--tt-nav-badge-bg);
    }

    &__badge-dot {
      position: absolute;
      top: 6px;
      right: 6px;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: #f97066;
    }

    &__footer {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px 10px 0;
      border-top: 1px solid var(--tt-nav-border);
      font-size: 12px;
      color: var(--tt-nav-muted);
    }

    &__about {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 4px 0;
      border: 0;
      background: transparent;
      font: inherit;
      font-size: 12px;
      color: var(--tt-nav-text);
      cursor: pointer;

      &:hover {
        color: var(--tt-nav-strong);
      }

      &:focus-visible {
        outline: 2px solid var(--tt-nav-accent);
        outline-offset: 2px;
      }
    }

    &__meta {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    &__toggle {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      border: 1px solid var(--tt-nav-border);
      border-radius: 6px;
      background: transparent;
      font-size: 16px;
      line-height: 1;
      color: var(--tt-nav-text);
      cursor: pointer;

      &:hover {
        background-color: var(--tt-nav-hover-bg);
      }

      &:focus-visible {
        outline: 2px solid var(--tt-nav-accent);
      }
    }

    &--rail {
      padding-inline: 6px;

      .app-sidebar__brand {
        justify-content: center;
        padding-inline: 0;
      }

      .app-sidebar__item {
        justify-content: center;
        padding-inline: 0;
      }

      .app-sidebar__about {
        justify-content: center;
      }

      .app-sidebar__footer {
        padding-inline: 0;
        align-items: center;
      }

      .app-sidebar__meta {
        flex-direction: column;
        gap: 6px;
      }

      .app-sidebar__version {
        font-size: 10px;
      }
    }
  }
</style>
