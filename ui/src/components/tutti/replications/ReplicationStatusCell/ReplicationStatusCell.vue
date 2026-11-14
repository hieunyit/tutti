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
  import { useI18n } from 'vue-i18n';
  import { computed } from 'vue';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import type { StatusVariant } from '@/components/ui/status';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';
  import type { AddId } from '@/utils/types/helper';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import { ReplicationsHelper } from '@/utils/helpers/ReplicationsHelper';

  const { t } = useI18n({
    messages: i18nReplications,
  });

  const props = defineProps<{
    replication: AddId<TuttiReplication>;
  }>();

  const behind = computed<number>(() =>
    ReplicationsHelper.getLiveReplicationEventsDifference(props.replication),
  );

  /** one primary status: paused beats initial sync beats lag */
  const primary = computed<{ variant: StatusVariant; label: string }>(() => {
    const { isPaused, isInitDone } = props.replication;

    if (isPaused) return { variant: 'warning', label: t('statusPaused') };

    if (!isInitDone) return { variant: 'info', label: t('statusInitial') };

    return behind.value > 0
      ? { variant: 'warning', label: t('statusLagging') }
      : { variant: 'success', label: t('statusUptodate') };
  });

  const initialText = computed<string>(() => {
    const { isInitDone, initObjDone, initObjListed } = props.replication;

    return isInitDone
      ? `${t('initialReplication')}: ${t('statusDone')}`
      : `${t('initialReplication')}: ${initObjDone} / ${initObjListed} ${t('labelObjects').toLowerCase()}`;
  });

  const liveText = computed<string | null>(() => {
    const { isInitDone, eventLag } = props.replication;

    if (!isInitDone) return null;

    const lag = eventLag ? GeneralHelper.formatDurationSeconds(eventLag) : null;
    const base =
      behind.value > 0
        ? `${behind.value} ${t('statusBehind').toLowerCase()}`
        : t('statusUptodate').toLowerCase();

    return `${t('liveReplication')}: ${base}${lag ? ` · ${t('labelReplicationLatency').toLowerCase()} ${lag}` : ''}`;
  });
</script>

<template>
  <div class="replication-status-cell">
    <StatusTag
      :variant="primary.variant"
      :label="primary.label"
    />
    <span class="replication-status-cell__secondary">{{ initialText }}</span>
    <span
      v-if="liveText"
      class="replication-status-cell__secondary"
      >{{ liveText }}</span
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
      font-size: 12px;
      color: var(--tt-text-3);
      white-space: normal;
      overflow-wrap: anywhere;
    }
  }
</style>
