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
  /** One sidebar link: icon, label, optional badge; an icon-only rail shows the label as a tooltip. */
  import { NTooltip } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nLayout from '../i18nLayout';
  import type { NavItem } from '@/utils/constants/navigation';

  defineProps<{
    item: NavItem;
    label: string;
    active: boolean;
    /** icon-only rail */
    rail: boolean;
    /** live counter for the item's badge (0 or undefined: no badge) */
    badge?: number;
  }>();

  const emit = defineEmits<{
    (e: 'navigate'): void;
  }>();

  const { t } = useI18n({ messages: i18nLayout });
</script>

<template>
  <NTooltip
    placement="right"
    :disabled="!rail"
  >
    <template #trigger>
      <RouterLink
        class="nav-item"
        :class="{ 'nav-item--active': active, 'nav-item--rail': rail }"
        :to="{ name: item.routeName }"
        :aria-label="label"
        :aria-current="active ? 'page' : undefined"
        @click="emit('navigate')"
      >
        <svg
          class="nav-item__icon"
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
          v-if="!rail"
          class="nav-item__label"
          >{{ label }}</span
        >
        <template v-if="badge">
          <span
            v-if="!rail"
            class="nav-item__badge"
          >
            {{ badge }}
            <span class="tt-sr-only">{{ t('badgeLabel') }}</span>
          </span>
          <span
            v-else
            class="nav-item__dot"
            aria-hidden="true"
          />
        </template>
      </RouterLink>
    </template>
    {{ label }}
  </NTooltip>
</template>

<style lang="scss" scoped>
  .nav-item {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    box-sizing: border-box;
    width: 100%;
    min-height: 36px;
    padding: 8px 10px;
    border-radius: var(--tt-radius-control);
    font-size: 14px;
    line-height: 20px;
    font-weight: 500;
    color: var(--tt-nav-text);
    text-decoration: none;
    transition: background-color var(--tt-duration-fast) var(--tt-ease);

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
      color: var(--tt-nav-strong);
      font-weight: 600;
      box-shadow: inset 2px 0 0 var(--tt-nav-accent);
    }

    &--rail {
      justify-content: center;
      padding-inline: 0;
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

    &__dot {
      position: absolute;
      top: 6px;
      right: 6px;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: var(--tt-nav-badge-bg);
      border: 1px solid var(--tt-nav-text);
    }
  }
</style>
