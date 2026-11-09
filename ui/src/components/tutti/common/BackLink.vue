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
  import type { RouteLocationRaw } from 'vue-router';

  defineProps<{
    to: RouteLocationRaw;
    label: string;
  }>();
</script>

<template>
  <RouterLink
    class="back-link"
    :to="to"
    ><span aria-hidden="true">‹</span> {{ label }}</RouterLink
  >
</template>

<style lang="scss" scoped>
  .back-link {
    align-self: flex-start;
    font-size: 13px;
    font-weight: 600;
    color: var(--tt-accent);
    text-decoration: none;

    &:hover {
      color: var(--tt-accent-hover);
    }
  }
</style>
