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
  defineProps<{
    title: string;
    description?: string;
  }>();
</script>

<template>
  <header class="page-header">
    <div class="page-header__row">
      <div class="page-header__text">
        <h1 class="page-header__title">{{ title }}</h1>
        <p
          v-if="description"
          class="page-header__description"
        >
          {{ description }}
        </p>
      </div>

      <div
        v-if="$slots.actions"
        class="page-header__actions"
      >
        <slot name="actions" />
      </div>
    </div>

    <div
      v-if="$slots.extra"
      class="page-header__extra"
    >
      <slot name="extra" />
    </div>
  </header>
</template>

<style lang="scss" scoped>
  .page-header {
    margin-bottom: 24px;

    &__row {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      flex-wrap: wrap;
      gap: 12px 16px;
    }

    &__text {
      min-width: 0;
      flex: 1 1 280px;
    }

    &__title {
      margin: 0;
      font-size: 24px;
      line-height: 32px;
      font-weight: 600;
      color: var(--text-color-1);
    }

    &__description {
      margin: 4px 0 0;
      font-size: 14px;
      line-height: 20px;
      color: var(--text-color-3);
      max-width: 72ch;
    }

    &__actions {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
      margin-left: auto;
    }

    &__extra {
      margin-top: 16px;
    }
  }
</style>
