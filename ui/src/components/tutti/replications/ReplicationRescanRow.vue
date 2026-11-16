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
   * The "Rescan" row of the replication summary: interval, last and next scan,
   * "Sync now" and a small control to change or turn off the interval.
   * Mutating controls are write-gated.
   */
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NSelect } from 'naive-ui';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
  import {
    RESCAN_MINUTES,
    ReplicationRescanHelper as H,
    type RescanBlock,
  } from '@/utils/helpers/ReplicationRescanHelper';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  const props = defineProps<{
    replication: TuttiReplication;
    /** starts a scan; rejects with the server error */
    sync: () => Promise<void>;
    /** seconds, 0 = off; rejects with the server error */
    changeInterval: (seconds: number) => Promise<void>;
  }>();

  const { t, locale } = useI18n({ messages: i18nReplicationRescan });

  const busy = ref(false);
  const editing = ref(false);
  const draft = ref(0);
  const feedback = ref<{ ok: boolean; text: string } | null>(null);

  const seconds = computed(() =>
    H.parseSeconds(props.replication.rescanInterval),
  );
  const intervalText = computed(() =>
    seconds.value
      ? t('detailRescanEvery', {
          interval: H.formatSeconds(seconds.value, t('unitMin'), t('unitHour')),
        })
      : t('detailRescanOff'),
  );
  const lastText = computed(() => {
    const time = H.formatShort(
      props.replication.lastRescanAt,
      String(locale.value),
    );

    if (!time) return '';

    const n = Number(props.replication.lastRescanObjects ?? 0);

    return `${t('detailLastScan', { time })} · ${t('detailLastScanObjects', { n }, n)}`;
  });
  const nextText = computed(() => {
    const time = H.formatShort(
      props.replication.nextRescanAt,
      String(locale.value),
    );

    return time ? t('detailNextScan', { time }) : '';
  });
  const block = computed<RescanBlock | null>(() =>
    H.blockedBy(props.replication),
  );
  const blockText = computed(() => {
    const keys: Record<RescanBlock, string> = {
      paused: 'syncBlockedPaused',
      initial: 'syncBlockedInitial',
      running: 'syncBlockedRunning',
    };

    return block.value ? t(keys[block.value]) : '';
  });
  const options = computed(() =>
    RESCAN_MINUTES.map((m) => ({
      label: m === 0 ? t('rescanOff') : t(`rescanOpt${m}`),
      value: m,
    })),
  );

  async function run(action: () => Promise<void>, ok: string, fail: string) {
    busy.value = true;
    feedback.value = null;

    try {
      await action();
      feedback.value = { ok: true, text: ok };

      return true;
    } catch (error) {
      feedback.value = { ok: false, text: H.errorText(error, fail) };

      return false;
    } finally {
      busy.value = false;
    }
  }

  function startEdit() {
    draft.value = seconds.value / 60;
    editing.value = true;
    feedback.value = null;
  }

  async function save() {
    const saved = await run(
      () => props.changeInterval(draft.value * 60),
      t('rescanSaved'),
      t('rescanSaveFailed'),
    );

    if (saved) editing.value = false;
  }
</script>

<template>
  <div class="rescan-row">
    <dt>{{ t('detailRescan') }}</dt>
    <dd>
      <span>{{ intervalText }}</span>
      <span v-if="replication.rescanRunning">{{ t('detailScanning') }}</span>
      <span v-if="lastText">{{ lastText }}</span>
      <span
        v-if="replication.lastRescanError"
        class="rescan-row__warn"
        >{{
          t('detailLastScanFailed', { error: replication.lastRescanError })
        }}</span
      >
      <span v-if="nextText">{{ nextText }}</span>
      <WriteGate>
        <div class="rescan-row__actions">
          <NButton
            size="small"
            :disabled="block !== null || busy"
            :title="blockText || undefined"
            :aria-describedby="block ? 'rescan-block-reason' : undefined"
            @click="run(sync, t('syncStarted'), t('syncFailed'))"
            >{{ t('syncNow') }}</NButton
          >
          <span
            v-if="blockText"
            id="rescan-block-reason"
            class="rescan-row__hint"
            >{{ blockText }}</span
          >
          <NButton
            v-if="!editing"
            size="small"
            quaternary
            :disabled="busy"
            @click="startEdit"
            >{{ t('rescanChange') }}</NButton
          >
        </div>
        <div
          v-if="editing"
          class="rescan-row__edit"
        >
          <NSelect
            v-model:value="draft"
            :options="options"
            size="small"
            :aria-label="t('rescanChangeLabel')"
          />
          <NButton
            size="small"
            type="primary"
            :disabled="busy"
            @click="save"
            >{{ t('rescanSave') }}</NButton
          >
          <NButton
            size="small"
            :disabled="busy"
            @click="editing = false"
            >{{ t('rescanCancel') }}</NButton
          >
        </div>
      </WriteGate>
      <span
        v-if="feedback"
        :class="feedback.ok ? 'rescan-row__ok' : 'rescan-row__warn'"
        :role="feedback.ok ? 'status' : 'alert'"
        >{{ feedback.text }}</span
      >
    </dd>
  </div>
</template>

<style lang="scss" scoped>
  .rescan-row {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);
    min-width: 0;
    grid-column: 1 / -1;

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

    &__actions,
    &__edit {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-2);
    }

    &__edit :deep(.c-select) {
      width: 200px;
      max-width: 100%;
    }

    &__hint {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }

    &__warn {
      color: var(--tt-warning, #b7791f);
    }
  }
</style>
