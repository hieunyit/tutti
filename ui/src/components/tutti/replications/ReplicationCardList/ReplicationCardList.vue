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
  /** Replications as cards on narrow screens: name link, status, actions. */
  import ReplicationNameCell from '@/components/tutti/replications/ReplicationNameCell/ReplicationNameCell.vue';
  import ReplicationRowMenu from '@/components/tutti/replications/ReplicationRowMenu/ReplicationRowMenu.vue';
  import ReplicationStatusCell from '@/components/tutti/replications/ReplicationStatusCell/ReplicationStatusCell.vue';
  import { ReplicationDetailHelper } from '@/utils/helpers/ReplicationDetailHelper';
  import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';
  import type { RowAction } from '@/utils/composables/useRowActions';
  import type { AddId } from '@/utils/types/helper';
  import { RouteName } from '@/utils/types/router';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  type Row = AddId<TuttiReplication>;

  defineProps<{
    rows: Row[];
    rename: (row: Row, name: string) => Promise<void>;
    actionsFor: (row: Row) => RowAction[];
    allBucketsText: string;
  }>();

  const to = (row: Row) => ({
    name: RouteName.TUTTI_REPLICATION_DETAIL,
    query: ReplicationDetailHelper.toQuery(row.id),
  });
</script>

<template>
  <ul class="replication-cards">
    <li
      v-for="row in rows"
      :key="row.idStr"
      class="replication-cards__item"
    >
      <div class="replication-cards__head">
        <ReplicationNameCell
          :replication="row"
          :rename="(name: string) => rename(row, name)"
          :detail-to="to(row)"
        />
        <ReplicationRowMenu
          :actions="actionsFor(row)"
          :label="ReplicationNameHelper.getDisplayName(row, allBucketsText)"
        />
      </div>
      <ReplicationStatusCell :replication="row" />
    </li>
  </ul>
</template>

<style lang="scss" scoped>
  .replication-cards {
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
