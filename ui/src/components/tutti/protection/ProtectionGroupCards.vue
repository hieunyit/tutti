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
  import { RouterLink } from 'vue-router';
  import type { ProtectionRow } from './protectionRows';
  import { StatusBadge } from '@/components/kit';
  import { RouteName } from '@/utils/types/router';

  defineProps<{ rows: ProtectionRow[] }>();
</script>

<template>
  <ul class="pg-cards">
    <li
      v-for="row in rows"
      :key="row.id"
      class="pg-cards__item"
    >
      <div class="pg-cards__head">
        <RouterLink
          class="pg-cards__name tt-mono"
          :to="{
            name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
            params: { name: row.id },
          }"
          >{{ row.id }}</RouterLink
        >
        <StatusBadge
          :tone="row.tone"
          :label="row.state"
        />
      </div>
      <small class="pg-cards__line tt-mono">{{ row.storages }}</small>
      <small class="pg-cards__line">{{ row.covers }} · {{ row.copies }}</small>
    </li>
  </ul>
</template>

<style scoped lang="scss">
  .pg-cards {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;
    padding: 0;
    list-style: none;

    &__item {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--tt-space-1);
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
    }

    &__head {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-2);
      width: 100%;
    }

    &__name {
      color: var(--tt-accent);
      font-weight: var(--tt-weight-strong);
      text-decoration: none;
    }

    &__line {
      max-width: 100%;
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
      overflow-wrap: anywhere;
    }
  }
</style>
