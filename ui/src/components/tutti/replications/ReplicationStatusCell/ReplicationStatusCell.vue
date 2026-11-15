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
  /**
   * One status badge (paused beats initial copy beats lag) with detail only
   * when it matters: copied bytes and objects while the initial copy runs,
   * the backlog only when events are behind.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { StatusBadge } from '@/components/kit';
  import { getReplicationStatus } from '@/components/tutti/replications/replicationStatus';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import ReplicationInitProgress from '@/components/tutti/replications/ReplicationInitProgress/ReplicationInitProgress.vue';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import type { AddId } from '@/utils/types/helper';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  const { t } = useI18n({ messages: i18nReplications });

  const props = defineProps<{
    replication: AddId<TuttiReplication>;
  }>();

  const primary = computed(() => getReplicationStatus(props.replication, t));

  const backlog = computed<string | null>(() => {
    if (primary.value.behind <= 0) return null;

    const lag = props.replication.eventLag
      ? GeneralHelper.formatDurationSeconds(props.replication.eventLag)
      : null;
    const events = t('eventsBehind', { n: primary.value.behind });

    return lag ? `${events} · ${lag}` : events;
  });
</script>

<template>
  <div class="replication-status-cell">
    <StatusBadge
      :tone="primary.tone"
      :label="primary.label"
    />
    <ReplicationInitProgress
      v-if="!replication.isInitDone"
      :replication="replication"
    />
    <span
      v-if="backlog"
      class="replication-status-cell__secondary"
      >{{ backlog }}</span
    >
  </div>
</template>

<style lang="scss" scoped>
  .replication-status-cell {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    min-width: 0;

    &__secondary {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
      overflow-wrap: anywhere;
    }
  }
</style>
