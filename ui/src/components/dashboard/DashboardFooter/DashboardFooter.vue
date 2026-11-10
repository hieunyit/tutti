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
  import { APP_VERSION } from '@/utils/constants/env';
</script>

<template>
  <footer class="dashboard-footer">
    <span>Tutti v{{ APP_VERSION }}</span>
  </footer>
</template>

<style lang="scss" scoped>
  .dashboard-footer {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 12px 24px;
    font-size: 12px;
    line-height: 16px;
    color: var(--tt-text-3);

    @media screen and (max-width: 639px) {
      padding: 12px 16px;
    }
  }
</style>
