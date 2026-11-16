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
   * Progress of one replication, once: the initial copy by copied bytes
   * (with speed and time left) and objects, then the live backlog. Nothing
   * that the header already says (name, direction, status).
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NProgress } from 'naive-ui';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import i18nReplicationSince from '@/components/tutti/replications/i18nReplicationSince';
  import { ReplicationSinceHelper } from '@/utils/helpers/ReplicationSinceHelper';
  import i18nReplicationInit from '@/components/tutti/replications/i18nReplicationInit';
  import { getReplicationStatus } from '@/components/tutti/replications/replicationStatus';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import { InitProgressHelper as H } from '@/utils/helpers/InitProgressHelper';
  import { mergeMessages } from '@/utils/helpers/mergeMessages';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  const props = defineProps<{ replication: TuttiReplication }>();

  const { t, locale } = useI18n({
    messages: mergeMessages(
      i18nReplications,
      i18nReplicationInit,
      i18nReplicationSince,
    ),
  });

  const progress = computed(() => H.fromReplication(props.replication));
  const status = computed(() => getReplicationStatus(props.replication, t));
  const copied = computed(() => {
    const p = progress.value;
    const text = t('initCopied', { size: H.formatSize(p.doneBytes) });

    return p.bytesPerSecond > 0
      ? `${text} · ${H.formatSize(p.bytesPerSecond)}/s`
      : text;
  });
  const eta = computed(() => H.formatDuration(progress.value.etaSeconds));
  const objects = computed(() =>
    t('initObjectsCount', {
      done: Number(props.replication.initObjDone).toLocaleString(locale.value),
      total: Number(props.replication.initObjListed).toLocaleString(
        locale.value,
      ),
    }),
  );
  const sinceText = computed(() => {
    const date = ReplicationSinceHelper.parseStored(props.replication.since);

    return date
      ? t('detailSinceValue', {
          date: ReplicationSinceHelper.formatMoment(date, String(locale.value)),
        })
      : '';
  });
  const lag = computed(() =>
    props.replication.eventLag
      ? GeneralHelper.formatDurationSeconds(props.replication.eventLag)
      : null,
  );
</script>

<template>
  <dl class="replication-summary">
    <template v-if="!replication.isInitDone">
      <div class="replication-summary__row replication-summary__row--wide">
        <dt>{{ t('copyProgress') }}</dt>
        <dd>
          <NProgress
            v-if="progress.tracked"
            type="line"
            :percentage="progress.percent ?? 0"
            :show-indicator="false"
            :height="8"
            :aria-label="t('copyProgress')"
          />
          <span v-if="progress.tracked">{{ copied }}</span>
          <span>{{ objects }}</span>
          <span v-if="eta">{{ t('timeLeft', { time: eta }) }}</span>
        </dd>
      </div>
    </template>
    <div
      v-else
      class="replication-summary__row"
    >
      <dt>{{ t('initialCopy') }}</dt>
      <dd>{{ t('statusDone') }} · {{ objects }}</dd>
    </div>
    <div
      v-if="replication.isInitDone"
      class="replication-summary__row"
    >
      <dt>{{ t('backlog') }}</dt>
      <dd>
        {{
          status.behind > 0
            ? t('eventsBehind', { n: status.behind })
            : t('statusUptodate')
        }}
        <template v-if="lag && status.behind > 0">· {{ lag }}</template>
      </dd>
    </div>
    <div
      v-if="sinceText"
      class="replication-summary__row"
    >
      <dt>{{ t('detailSince') }}</dt>
      <dd>{{ sinceText }}</dd>
    </div>
    <div class="replication-summary__row">
      <dt>{{ t('createdAt') }}</dt>
      <dd>{{ formatDateTime(replication.createdAt) }}</dd>
    </div>
  </dl>
</template>

<style lang="scss" scoped>
  .replication-summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--tt-space-4);
    margin: 0;

    &__row {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      min-width: 0;

      &--wide {
        grid-column: 1 / -1;
      }
    }

    dt {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }

    dd {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      margin: 0;
      overflow-wrap: anywhere;
    }
  }
</style>
