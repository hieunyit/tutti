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
  /** Routing rules as cards on narrow screens (a table would crush the text). */
  import { useI18n } from 'vue-i18n';
  import RoutingRuleMenu from './RoutingRuleMenu.vue';
  import { ALL_BUCKETS } from './routingColumns';
  import { routingRuleKey } from './useRoutingRuleActions';
  import { StatusBadge } from '@/components/kit';
  import i18nRouting from '@/pages/TuttiRoutingPage/i18nRouting';
  import type { RowAction } from '@/utils/composables/useRowActions';
  import type { RoutingPolicy } from '@/utils/types/tuttiCore';

  defineProps<{
    rows: RoutingPolicy[];
    actionsFor: (row: RoutingPolicy) => RowAction[];
  }>();

  const { t } = useI18n({ messages: i18nRouting });
</script>

<template>
  <ul class="rule-cards">
    <li
      v-for="row in rows"
      :key="routingRuleKey(row)"
      class="rule-cards__item"
    >
      <div class="rule-cards__main">
        <strong class="rule-cards__user">{{ row.user }}</strong>
        <small class="rule-cards__detail">
          {{ row.bucket === ALL_BUCKETS ? t('allBuckets') : row.bucket }}
          &middot;
          {{ row.toStorage || t('noStorage') }}
        </small>
        <StatusBadge
          :tone="row.isBlocked ? 'danger' : 'success'"
          :label="row.isBlocked ? t('statusBlocked') : t('statusAllowed')"
        />
      </div>
      <RoutingRuleMenu
        :actions="actionsFor(row)"
        :label="row.user"
      />
    </li>
  </ul>
</template>

<style scoped lang="scss">
  .rule-cards {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;
    padding: 0;
    list-style: none;

    &__item {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: var(--tt-space-2);
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
    }

    &__main {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--tt-space-1);
      min-width: 0;
    }

    &__user {
      overflow-wrap: anywhere;
    }

    &__detail {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
      overflow-wrap: anywhere;
    }
  }
</style>
