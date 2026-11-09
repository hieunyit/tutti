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

  /** message parameters; the names the operator looks for are rendered bold */
  function params(item: AttentionItem): Record<string, string | number> {
    return item.kind === 'proxyStale'
      ? { ...item.params, since: formatRelative(String(item.params.since)) }
      : item.params;
  }
</script>

<template>
  <section
    v-if="items.length > 0"
    class="dashboard-attention"
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
        <span
          class="dashboard-attention__tag"
          :class="`dashboard-attention__tag--${item.severity}`"
        >
          {{
            item.severity === 'error'
              ? t('severityError')
              : t('severityWarning')
          }}
        </span>
        <i18n-t
          :keypath="item.kind"
          tag="span"
          scope="parent"
          class="dashboard-attention__text"
        >
          <template
            v-for="(value, key) in params(item)"
            :key="key"
            #[key]
          >
            <strong>{{ value }}</strong>
          </template>
        </i18n-t>
        <RouterLink
          class="dashboard-attention__link"
          :to="{ name: item.route }"
        >
          {{ t(`action_${item.kind}`) }}
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
    overflow: hidden;
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);

    &__header {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px var(--tt-card-padding);
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    &__title {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
    }

    &__count {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__list {
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__item {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px 12px;
      padding: 12px var(--tt-card-padding);

      & + & {
        border-top: 1px solid var(--tt-border-subtle);
      }
    }

    &__tag {
      padding: 2px 8px;
      border: 1px solid;
      border-radius: var(--tt-radius-tag);
      font-size: 12px;
      line-height: 16px;
      font-weight: 600;

      &--error {
        color: var(--tt-danger);
        background-color: var(--tt-danger-bg);
        border-color: var(--tt-danger-border);
      }

      &--warning {
        color: var(--tt-warning);
        background-color: var(--tt-warning-bg);
        border-color: var(--tt-warning-border);
      }
    }

    &__text {
      flex: 1 1 320px;
      min-width: 0;
      font-size: 14px;
      overflow-wrap: anywhere;
    }

    &__link {
      font-size: 14px;
      font-weight: 600;
      color: var(--tt-accent);
      text-decoration: none;

      &:hover {
        color: var(--tt-accent-hover);
        text-decoration: underline;
      }
    }

    &__more {
      width: 100%;
      padding: 10px var(--tt-card-padding);
      border: 0;
      border-top: 1px solid var(--tt-border-subtle);
      background: transparent;
      font: inherit;
      font-size: 13px;
      font-weight: 600;
      color: var(--tt-accent);
      text-align: left;
      cursor: pointer;

      &:hover {
        background-color: var(--tt-subtle);
      }
    }
  }
</style>
