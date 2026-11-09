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
  import { NTag } from 'naive-ui';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDashboard from './i18nDashboard';
  import type { AttentionItem } from '@/utils/helpers/DashboardHelper';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';

  const VISIBLE_ITEMS = 5;

  const props = defineProps<{
    items: AttentionItem[];
  }>();

  const { t } = useI18n({ messages: i18nDashboard });
  const isExpanded = ref(false);

  const visibleItems = computed(() =>
    isExpanded.value ? props.items : props.items.slice(0, VISIBLE_ITEMS),
  );
  const hiddenCount = computed(() =>
    Math.max(0, props.items.length - VISIBLE_ITEMS),
  );
  const hasError = computed(() =>
    props.items.some((item) => item.severity === 'error'),
  );

  function text(item: AttentionItem): string {
    const params =
      item.kind === 'proxyStale'
        ? { ...item.params, since: formatRelative(String(item.params.since)) }
        : item.params;

    return t(item.kind, params);
  }
</script>

<template>
  <section
    v-if="items.length > 0"
    class="dashboard-attention"
    :class="`dashboard-attention--${hasError ? 'error' : 'warning'}`"
    :aria-label="t('attentionTitle')"
  >
    <header class="dashboard-attention__header">
      <h2 class="dashboard-attention__title">{{ t('attentionTitle') }}</h2>
      <span class="dashboard-attention__count">
        {{ t('attentionCount', { n: items.length }, items.length) }}
      </span>
    </header>

    <ul class="dashboard-attention__list">
      <li
        v-for="item in visibleItems"
        :key="item.id"
        class="dashboard-attention__item"
      >
        <NTag
          :type="item.severity"
          size="small"
          :bordered="false"
          class="dashboard-attention__tag"
        >
          {{
            item.severity === 'error'
              ? t('severityError')
              : t('severityWarning')
          }}
        </NTag>
        <span class="dashboard-attention__text">{{ text(item) }}</span>
        <RouterLink
          class="dashboard-attention__link"
          :to="{ name: item.route }"
        >
          {{ t('open') }} →
        </RouterLink>
      </li>
    </ul>

    <button
      v-if="hiddenCount > 0"
      type="button"
      class="dashboard-attention__more"
      @click="isExpanded = !isExpanded"
    >
      {{ isExpanded ? t('showLess') : t('showMore', { n: hiddenCount }) }}
    </button>
  </section>
</template>

<style lang="scss" scoped>
  .dashboard-attention {
    padding: 12px 16px;
    border: 1px solid var(--border-color);
    border-left-width: 4px;
    border-radius: 8px;
    background-color: var(--card-color);

    &--error {
      border-left-color: var(--error-color);
    }

    &--warning {
      border-left-color: var(--warning-color);
    }

    &__header {
      display: flex;
      align-items: baseline;
      gap: 8px;
      margin-bottom: 4px;
    }

    &__title {
      margin: 0;
      font-size: 15px;
      font-weight: 600;
      color: var(--text-color-1);
    }

    &__count {
      font-size: 13px;
      color: var(--text-color-3);
    }

    &__list {
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__item {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 8px 0;
      font-size: 14px;

      & + & {
        border-top: 1px solid var(--border-color);
      }

      @media screen and (max-width: 640px) {
        flex-wrap: wrap;
        gap: 4px 12px;
      }
    }

    &__tag {
      flex: none;
      min-width: 64px;
      justify-content: center;
    }

    &__text {
      flex: 1 1 auto;
      min-width: 0;
      overflow-wrap: anywhere;
      color: var(--text-color-1);

      @media screen and (max-width: 640px) {
        flex-basis: calc(100% - 90px);
      }
    }

    &__link {
      flex: none;
      margin-left: auto;
      white-space: nowrap;
      color: var(--primary-color);
      text-decoration: none;

      &:hover,
      &:focus-visible {
        text-decoration: underline;
      }
    }

    &__more {
      margin-top: 4px;
      padding: 4px 0;
      border: 0;
      background: none;
      font: inherit;
      font-size: 13px;
      cursor: pointer;
      color: var(--primary-color);

      &:hover {
        text-decoration: underline;
      }
    }
  }
</style>
