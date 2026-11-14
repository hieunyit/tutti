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
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nReplicationInit from '@/components/tutti/replications/i18nReplicationInit';
  import { InitProgressHelper as H } from '@/utils/helpers/InitProgressHelper';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  const { t } = useI18n({ messages: i18nReplicationInit });

  const props = defineProps<{
    replication: TuttiReplication;
  }>();

  const progress = computed(() => H.fromReplication(props.replication));

  const bytesText = computed<string>(() => {
    const p = progress.value;
    const total = `${p.listingComplete ? '' : `${t('initAtLeast')} `}${H.formatSize(p.totalBytes)}`;
    const pct = p.percent === null ? '' : ` (${p.percent}%)`;
    const speed =
      p.bytesPerSecond > 0
        ? `${H.formatSize(p.bytesPerSecond)}/s`
        : t('initSpeedUnknown');
    const duration = H.formatDuration(p.etaSeconds);
    const eta = duration
      ? t('initEtaLeft', { eta: duration })
      : t('initEtaUnknown');

    return `${H.formatSize(p.doneBytes)} / ${total}${pct} · ${speed} · ${eta}`;
  });

  const objectsText = computed<string>(() =>
    t('initObjectsCount', {
      done: props.replication.initObjDone,
      total: props.replication.initObjListed,
    }),
  );
</script>

<template>
  <div class="replication-init-progress">
    <template v-if="progress.tracked">
      <div
        class="replication-init-progress__bar"
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="progress.percent ?? 0"
      >
        <div
          class="replication-init-progress__fill"
          :style="{ width: `${progress.percent ?? 0}%` }"
        />
      </div>
      <span>{{ bytesText }}</span>
    </template>
    <span>{{ objectsText }}</span>
  </div>
</template>

<style lang="scss" scoped>
  .replication-init-progress {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: 12px;
    color: var(--tt-text-3);
    overflow-wrap: anywhere;

    &__bar {
      height: 4px;
      max-width: 240px;
      border-radius: 2px;
      background: var(--tt-border);
      overflow: hidden;
    }

    &__fill {
      height: 100%;
      background: var(--tt-accent);
    }
  }
</style>
