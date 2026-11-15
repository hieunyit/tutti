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
   * Deprecated alias of the kit StatusBadge (same look, one implementation).
   * New code imports StatusBadge from '@/components/kit'.
   */
  import type { StatusVariant } from './status';
  import StatusBadge from '@/components/kit/StatusBadge.vue';

  withDefaults(
    defineProps<{
      variant?: StatusVariant;
      /** text; the default slot wins when both are given */
      label?: string;
      /** hide the dot (the text still carries the meaning) */
      noDot?: boolean;
    }>(),
    { variant: 'neutral', label: undefined, noDot: false },
  );
</script>

<template>
  <StatusBadge
    :tone="variant"
    :label="label"
    :no-dot="noDot"
  >
    <slot>{{ label }}</slot>
  </StatusBadge>
</template>
