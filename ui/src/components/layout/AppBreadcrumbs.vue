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
  import i18nLayout from './i18nLayout';
  import { RouteName } from '@/utils/types/router';

  interface Crumb {
    label: string;
    to?: { name: RouteName; params?: Record<string, string> };
  }

  const { t } = useI18n({ messages: i18nLayout });
  const { t: tGlobal } = useI18n({ useScope: 'global' });
  const route = useRoute();

  const crumbs = computed<Crumb[]>(() => {
    const { navGroup, navSection } = route.meta;
    const name = route.name as RouteName | undefined;
    const list: Crumb[] = [];

    if (navGroup) {
      list.push({ label: t(`group.${navGroup}`) });
    }

    if (!name) return list;

    const entityName = [route.params.storageName, route.params.name].find(
      (value): value is string => typeof value === 'string' && value !== '',
    );
    const section = navSection ?? name;

    if (name === section) {
      list.push({ label: tGlobal(name) });

      return list;
    }

    list.push({
      label: tGlobal(section),
      to: { name: section },
    });

    if (
      (name === RouteName.CHORUS_STORAGE_DETAILS ||
        name === RouteName.TUTTI_USER_DETAIL ||
        name === RouteName.TUTTI_PROTECTION_GROUP_DETAIL) &&
      entityName
    ) {
      list.push({ label: entityName });
    } else {
      list.push({ label: tGlobal(name) });
    }

    return list;
  });
</script>

<template>
  <nav
    class="app-breadcrumbs"
    :aria-label="t('breadcrumbs')"
  >
    <ol class="app-breadcrumbs__list">
      <li
        v-for="(crumb, index) in crumbs"
        :key="`${index}-${crumb.label}`"
        class="app-breadcrumbs__item"
        :class="{
          'app-breadcrumbs__item--current': index === crumbs.length - 1,
        }"
      >
        <RouterLink
          v-if="crumb.to && index < crumbs.length - 1"
          class="app-breadcrumbs__link"
          :to="crumb.to"
          >{{ crumb.label }}</RouterLink
        >
        <span
          v-else
          :aria-current="index === crumbs.length - 1 ? 'page' : undefined"
          >{{ crumb.label }}</span
        >
      </li>
    </ol>
  </nav>
</template>

<style lang="scss" scoped>
  .app-breadcrumbs {
    min-width: 0;

    &__list {
      display: flex;
      align-items: center;
      margin: 0;
      padding: 0;
      list-style: none;
      font-size: 13px;
      color: var(--tt-text-3);
      white-space: nowrap;
      overflow: hidden;
    }

    &__item {
      display: flex;
      align-items: center;
      min-width: 0;

      &:not(:last-child)::after {
        content: '/';
        margin: 0 8px;
        color: var(--tt-text-3);
      }

      &--current {
        flex: 0 1 auto;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        color: var(--tt-text);
        font-weight: 600;
      }
    }

    &__link {
      color: inherit;
      text-decoration: none;

      &:hover {
        color: var(--tt-text);
        text-decoration: underline;
      }
    }

    @media screen and (max-width: 639px) {
      // phones: only the current page
      &__item:not(:last-child) {
        display: none;
      }
    }
  }
</style>
