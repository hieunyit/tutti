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
  /** Diff reports as cards on narrow screens: report link, status, actions. */
  import DiffReportsNameCell from '@/components/tutti/diff-reports/DiffReportsNameCell/DiffReportsNameCell.vue';
  import DiffReportsStatusCell from '@/components/tutti/diff-reports/DiffReportsStatusCell/DiffReportsStatusCell.vue';
  import ReplicationRowMenu from '@/components/tutti/replications/ReplicationRowMenu/ReplicationRowMenu.vue';
  import type { RowAction } from '@/utils/composables/useRowActions';
  import type { AddId } from '@/utils/types/helper';
  import type { DiffReport } from '@/utils/types/tuttiCore';

  type Row = AddId<DiffReport>;

  defineProps<{
    rows: Row[];
    actionsFor: (row: Row) => RowAction[];
    labelOf: (row: Row) => string;
  }>();
</script>

<template>
  <ul class="diff-cards">
    <li
      v-for="row in rows"
      :key="row.idStr"
      class="diff-cards__item"
    >
      <div class="diff-cards__head">
        <DiffReportsNameCell :report="row" />
        <ReplicationRowMenu
          :actions="actionsFor(row)"
          :label="labelOf(row)"
        />
      </div>
      <DiffReportsStatusCell
        v-if="row.locations.length === 2"
        :report="row"
      />
    </li>
  </ul>
</template>

<style lang="scss" scoped>
  .diff-cards {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;
    padding: 0;
    list-style: none;

    &__item {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
    }

    &__head {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: var(--tt-space-2);
      min-width: 0;
    }
  }
</style>
