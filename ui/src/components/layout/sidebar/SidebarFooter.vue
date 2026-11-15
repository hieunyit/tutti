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
  /** Sidebar footer: app version and (desktop only) the collapse toggle. */
  import { NTooltip } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nLayout from '../i18nLayout';
  import { APP_VERSION } from '@/utils/constants/env';

  defineProps<{
    collapsed: boolean;
    /** the drawer has no collapse toggle */
    isDrawer: boolean;
    rail: boolean;
    /** id of the element the toggle collapses (the nav) */
    controls?: string;
  }>();

  const emit = defineEmits<{
    (e: 'toggle'): void;
  }>();

  const { t } = useI18n({ messages: i18nLayout });
</script>

<template>
  <div
    class="sidebar-footer"
    :class="{ 'sidebar-footer--rail': rail }"
  >
    <span
      class="sidebar-footer__version"
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
          class="sidebar-footer__toggle"
          :aria-label="collapsed ? t('expand') : t('collapse')"
          :aria-expanded="!collapsed"
          :aria-controls="controls"
          @click="emit('toggle')"
        >
          <span aria-hidden="true">{{ collapsed ? '›' : '‹' }}</span>
        </button>
      </template>
      {{ collapsed ? t('expand') : t('collapse') }}
    </NTooltip>
  </div>
</template>

<style lang="scss" scoped>
  .sidebar-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 10px 0;
    border-top: 1px solid var(--tt-nav-border);
    font-size: 12px;
    color: var(--tt-nav-muted);

    &--rail {
      flex-direction: column;
      gap: 6px;
      padding-inline: 0;
    }

    &--rail &__version {
      font-size: 10px;
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
        outline-offset: 2px;
      }
    }
  }
</style>
