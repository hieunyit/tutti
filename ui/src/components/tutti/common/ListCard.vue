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
   * A list page card: SectionCard header, one flat filter bar (filters,
   * applied-filter tags, bulk actions) and the table running edge to edge.
   */
  import SectionCard from '@/components/ui/SectionCard.vue';

  defineProps<{
    title: string;
  }>();
</script>

<template>
  <SectionCard
    class="tt-list-card"
    :title="title"
    flush
  >
    <div class="tt-list-card__bar">
      <slot name="filters" />
      <div
        v-if="$slots.tags || $slots.actions"
        class="tt-list-card__toolbar"
      >
        <slot name="tags" />
        <div class="tt-list-card__actions">
          <slot name="actions" />
        </div>
      </div>
    </div>

    <div class="tt-table-wrap tt-list-card__table">
      <slot />
    </div>
  </SectionCard>
</template>

<style lang="scss" scoped>
  .tt-list-card {
    &__bar {
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: 12px var(--tt-card-padding);
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    &__toolbar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px 12px;
    }

    &__actions {
      display: flex;
      align-items: center;
    }

    &__table {
      padding: 0 var(--tt-card-padding);
    }
  }
</style>
