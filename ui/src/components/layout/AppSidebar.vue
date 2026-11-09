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
  import { useI18n } from 'vue-i18n';
  import { CIcon } from '@clyso/clyso-ui-kit';
  import { NTooltip } from 'naive-ui';
  import i18nLayout from './i18nLayout';
  import { NAV_GROUPS } from '@/utils/constants/navigation';
  import { useAboutDialog } from '@/utils/composables/useAboutDialog';
  import { APP_VERSION } from '@/utils/constants/env';
  import { IconName } from '@/utils/types/icon';
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
  const route = useRoute();
  const { openAbout } = useAboutDialog();

  const isRail = computed(() => Boolean(props.collapsed) && !props.isDrawer);

  const activeItem = computed<RouteName | undefined>(
    () => route.meta.navSection ?? (route.name as RouteName | undefined),
  );

  function handleAbout() {
    openAbout();
    emit('navigate');
  }
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
      <span class="app-sidebar__logo">T</span>
      <span
        v-if="!isRail"
        class="app-sidebar__product"
        >Tutti</span
      >
    </RouterLink>

    <nav
      class="app-sidebar__nav"
      :aria-label="t('navLabel')"
    >
      <section
        v-for="group in NAV_GROUPS"
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
              :aria-current="activeItem === item.routeName ? 'page' : undefined"
              @click="emit('navigate')"
            >
              <CIcon
                class="app-sidebar__icon"
                :name="item.icon"
                :width="18"
                :height="18"
              />
              <span
                v-if="!isRail"
                class="app-sidebar__label"
                >{{ $t(item.routeName) }}</span
              >
            </RouterLink>
          </template>
          {{ $t(item.routeName) }}
        </NTooltip>
      </section>
    </nav>

    <div class="app-sidebar__footer">
      <NTooltip
        placement="right"
        :disabled="!isRail"
      >
        <template #trigger>
          <button
            type="button"
            class="app-sidebar__item app-sidebar__about"
            @click="handleAbout"
          >
            <CIcon
              class="app-sidebar__icon"
              :name="IconName.BASE_CONTRACT"
              :width="18"
              :height="18"
            />
            <span
              v-if="!isRail"
              class="app-sidebar__label"
              >{{ t('about') }}</span
            >
          </button>
        </template>
        {{ t('about') }}
      </NTooltip>

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
              @click="emit('toggle')"
            >
              <CIcon
                :name="
                  collapsed
                    ? IconName.BASE_ARROW_FORWARD
                    : IconName.BASE_ARROW_BACK
                "
                :width="16"
                :height="16"
              />
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
    height: 100%;
    min-height: 0;
    width: 100%;
    background-color: var(--card-color);
    border-right: 1px solid var(--border-color);
    padding: 12px 8px;
    overflow: hidden;

    &__brand {
      display: flex;
      align-items: center;
      gap: 10px;
      height: 40px;
      padding: 0 8px;
      margin-bottom: 8px;
      color: var(--text-color-1);
      text-decoration: none;
    }

    &__logo {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 28px;
      height: 28px;
      border-radius: 8px;
      font-weight: 700;
      font-size: 16px;
      color: var(--card-color);
      background-color: var(--primary-color);
    }

    &__product {
      font-size: 18px;
      font-weight: 600;
      letter-spacing: 0.2px;
    }

    &__nav {
      flex: 1;
      min-height: 0;
      overflow-y: auto;
    }

    &__group {
      margin-bottom: 8px;
    }

    &__group-label {
      margin: 12px 0 4px;
      padding: 0 12px;
      font-size: 11px;
      line-height: 16px;
      font-weight: 600;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--text-color-3);
    }

    &__group-divider {
      height: 1px;
      margin: 10px 8px;
      background-color: var(--border-color);
    }

    &__item {
      display: flex;
      align-items: center;
      gap: 12px;
      width: 100%;
      height: 36px;
      padding: 0 12px;
      margin-bottom: 2px;
      border: 0;
      border-radius: 6px;
      background: transparent;
      font: inherit;
      font-size: 14px;
      color: var(--text-color-2);
      text-decoration: none;
      cursor: pointer;
      transition: background-color 0.15s;

      &:hover {
        background-color: var(--side-menu-option-hover-color);
      }

      &:focus-visible {
        outline: 2px solid var(--primary-color);
        outline-offset: -2px;
      }

      &--active {
        background-color: var(--side-menu-option-matched-color);
        color: var(--text-color-1);
        font-weight: 600;
        box-shadow: inset 3px 0 0 var(--primary-color);
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

    &__footer {
      border-top: 1px solid var(--border-color);
      padding-top: 8px;
    }

    &__meta {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 4px 8px 0 12px;
      font-size: 12px;
      color: var(--text-color-3);
    }

    &__toggle {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      border: 0;
      border-radius: 6px;
      background: transparent;
      color: var(--text-color-2);
      cursor: pointer;

      &:hover {
        background-color: var(--side-menu-option-hover-color);
      }

      &:focus-visible {
        outline: 2px solid var(--primary-color);
      }
    }

    &--rail {
      padding-inline: 6px;

      .app-sidebar__brand {
        justify-content: center;
        padding: 0;
      }

      .app-sidebar__item {
        justify-content: center;
        padding: 0;
      }

      .app-sidebar__meta {
        flex-direction: column;
        gap: 4px;
        padding: 4px 0 0;
      }

      .app-sidebar__version {
        font-size: 10px;
      }
    }
  }
</style>
