<!--
  - Copyright © 2026 Clyso GmbH
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
  import type { DiffReportStorageEntry } from '@/utils/types/tuttiCore';

  defineProps<{
    storageEntries: DiffReportStorageEntry[];
  }>();
</script>

<template>
  <ul class="diff-report-detail-entries-storage-cell">
    <li
      v-for="(entry, index) in storageEntries"
      :key="index"
    >
      {{ entry.storage }}/{{ entry.bucket }}
    </li>
  </ul>
</template>

<style lang="scss" scoped>
  .diff-report-detail-entries-storage-cell {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
</style>
