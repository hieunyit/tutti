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
   * Quiet secondary line of the list: "Rescans every 1 h", or "Scanning…"
   * while a scan runs. Nothing when the rescan is off.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
  import { ReplicationRescanHelper as H } from '@/utils/helpers/ReplicationRescanHelper';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  const props = defineProps<{ replication: TuttiReplication }>();

  const { t } = useI18n({ messages: i18nReplicationRescan });

  const seconds = computed(() =>
    H.parseSeconds(props.replication.rescanInterval),
  );
  const text = computed(() =>
    seconds.value
      ? t('listRescan', {
          interval: H.formatSeconds(seconds.value, t('unitMin'), t('unitHour')),
        })
      : '',
  );
  const running = computed(() => props.replication.rescanRunning === true);
</script>

<template>
  <span
    v-if="text"
    class="rescan-line"
    >{{ text }}</span
  >
  <span
    v-if="running"
    class="rescan-line rescan-line--running"
    role="status"
    >{{ t('listScanning') }}</span
  >
</template>

<style lang="scss" scoped>
  .rescan-line {
    overflow: hidden;
    font-size: 0.8125rem;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.75;

    &--running {
      opacity: 1;
      color: var(--tt-accent);
    }
  }
</style>
