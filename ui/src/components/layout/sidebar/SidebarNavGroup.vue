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
   * A sidebar group: a heading (or a thin divider in the rail, nothing for an
   * unlabeled group) and its items. Rendered as a labelled list so screen
   * readers announce "Connect, list, 2 items".
   */
  import { useId } from 'vue';
  import SidebarNavItem from './SidebarNavItem.vue';
  import type { NavBadgeKey, NavGroup } from '@/utils/constants/navigation';

  defineProps<{
    group: NavGroup;
    label: string;
    rail: boolean;
    /** route name of the active item */
    activeItem?: string;
    badges: Partial<Record<NavBadgeKey, number>>;
    /** translates an item's route name into its label */
    itemLabel: (routeName: string) => string;
  }>();

  const emit = defineEmits<{
    (e: 'navigate'): void;
  }>();

  const headingId = `nav-group-${useId()}`;
</script>

<template>
  <section class="nav-group">
    <h2
      v-if="!group.unlabeled && !rail"
      :id="headingId"
      class="nav-group__label"
    >
      {{ label }}
    </h2>
    <div
      v-else-if="!group.unlabeled"
      class="nav-group__divider"
      aria-hidden="true"
    />
    <ul
      class="nav-group__list"
      :aria-label="group.unlabeled || rail ? label : undefined"
      :aria-labelledby="group.unlabeled || rail ? undefined : headingId"
    >
      <li
        v-for="item in group.items"
        :key="item.routeName"
      >
        <SidebarNavItem
          :item="item"
          :label="itemLabel(item.routeName)"
          :active="activeItem === item.routeName"
          :rail="rail"
          :badge="item.badgeKey ? badges[item.badgeKey] : undefined"
          @navigate="emit('navigate')"
        />
      </li>
    </ul>
  </section>
</template>

<style lang="scss" scoped>
  .nav-group {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-top: 12px;

    &__label {
      margin: 0;
      padding: 0 10px 6px;
      font-size: 11px;
      line-height: 16px;
      font-weight: 600;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--tt-nav-muted);
    }

    &__divider {
      height: 1px;
      margin: 6px 8px;
      background-color: var(--tt-nav-border);
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: 2px;
      margin: 0;
      padding: 0;
      list-style: none;
    }
  }
</style>
