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
  /** One filter row inside a SectionCard: controls on the left, a note on the right. */
</script>

<template>
  <div class="tt-filter-bar">
    <slot />
    <span
      v-if="$slots.note"
      class="tt-filter-bar__note"
    >
      <slot name="note" />
    </span>
  </div>
</template>

<style lang="scss" scoped>
  .tt-filter-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    padding: 12px var(--tt-card-padding);
    border-bottom: 1px solid var(--tt-border-subtle);

    &__note {
      margin-left: auto;
      font-size: 12px;
      color: var(--tt-text-3);
    }
  }
</style>
