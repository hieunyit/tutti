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
   * Console style tail of the replication activity: dark monospace panel with
   * timestamps and coloured levels. Auto-scroll follows the newest line and
   * turns itself off when the user scrolls up. Lines are plain text, so the
   * selection and the copy button give exactly what is shown.
   */
  import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import ActivityConsoleToolbar from './ActivityConsoleToolbar.vue';
  import i18nReplicationActivity from './i18nReplicationActivity';
  import { Callout, EmptyBlock } from '@/components/kit';
  import {
    ActivityLogHelper,
    type ActivityLevelFilter,
  } from '@/utils/helpers/ActivityLogHelper';
  import type { ReplicationActivityEvent } from '@/utils/types/replicationActivity';

  const props = defineProps<{
    events: ReplicationActivityEvent[];
    live: boolean;
    /** the last poll failed */
    error: boolean;
    /** used in the downloaded file name */
    label: string;
  }>();

  const emit = defineEmits<{
    (e: 'update:live', value: boolean): void;
    (e: 'clear'): void;
    (e: 'retry'): void;
  }>();

  const { t } = useI18n({ messages: i18nReplicationActivity });

  const level = ref<ActivityLevelFilter>('all');
  const text = ref('');
  const autoScroll = ref(true);
  const notice = ref('');
  const scroller = ref<HTMLElement | null>(null);
  let noticeTimer: ReturnType<typeof setTimeout> | null = null;

  const visible = computed(() =>
    ActivityLogHelper.filter(props.events, level.value, text.value),
  );
  const lines = computed(() =>
    visible.value.map((e) => ({
      cursor: e.cursor,
      tone: ActivityLogHelper.tone(e.type),
      text: ActivityLogHelper.formatLine(e),
    })),
  );

  function scrollToEnd() {
    const el = scroller.value;

    if (el) el.scrollTop = el.scrollHeight;
  }

  watch(
    () => lines.value.length,
    async () => {
      if (!autoScroll.value) return;

      await nextTick();
      scrollToEnd();
    },
  );
  watch(autoScroll, async (on) => {
    if (!on) return;

    await nextTick();
    scrollToEnd();
  });

  function onScroll() {
    const el = scroller.value;

    if (!el || !autoScroll.value) return;

    // the user scrolled away from the end: stop following
    if (el.scrollHeight - el.scrollTop - el.clientHeight > 24) {
      autoScroll.value = false;
    }
  }

  function flash(message: string) {
    notice.value = message;

    if (noticeTimer) clearTimeout(noticeTimer);

    noticeTimer = setTimeout(() => {
      notice.value = '';
    }, 4000);
  }

  async function copy() {
    const content = ActivityLogHelper.toText(visible.value);

    try {
      await navigator.clipboard.writeText(content);
      flash(t('copied', { n: visible.value.length }));
    } catch {
      flash(t('copyFailed'));
    }
  }

  function download() {
    const blob = new Blob([`${ActivityLogHelper.toText(props.events)}\n`], {
      type: 'text/plain;charset=utf-8',
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');

    link.href = url;
    link.download = ActivityLogHelper.fileName(props.label);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  onBeforeUnmount(() => {
    if (noticeTimer) clearTimeout(noticeTimer);
  });
</script>

<template>
  <div class="activity-console">
    <ActivityConsoleToolbar
      v-model:auto-scroll="autoScroll"
      v-model:level="level"
      v-model:text="text"
      :live="live"
      :has-lines="events.length > 0"
      :has-visible="visible.length > 0"
      @update:live="emit('update:live', $event)"
      @clear="emit('clear')"
      @copy="copy"
      @download="download"
    />

    <Callout
      v-if="error"
      tone="danger"
      :title="t('loadErrorTitle')"
    >
      {{ t('loadErrorText') }}
      <template #actions>
        <NButton
          size="small"
          @click="emit('retry')"
          >{{ t('retry') }}</NButton
        >
      </template>
    </Callout>

    <div
      ref="scroller"
      class="activity-console__screen"
      role="log"
      aria-live="off"
      tabindex="0"
      :aria-label="t('consoleLabel')"
      @scroll.passive="onScroll"
    >
      <p
        v-for="line in lines"
        :key="line.cursor"
        class="activity-console__line"
        :class="`activity-console__line--${line.tone}`"
      >
        {{ line.text }}
      </p>
      <EmptyBlock
        v-if="events.length === 0"
        class="activity-console__empty"
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      />
      <p
        v-else-if="lines.length === 0"
        class="activity-console__line activity-console__line--dim"
      >
        {{ t('noMatch') }}
      </p>
    </div>

    <div class="activity-console__footer">
      <span>{{
        t('lineCount', { shown: visible.length, total: events.length })
      }}</span>
      <span role="status">{{ notice }}</span>
      <span>{{ t('consoleHint') }}</span>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .activity-console {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    min-width: 0;

    &__screen {
      min-height: 220px;
      max-height: 480px;
      padding: var(--tt-space-3);
      overflow: auto;
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12.5px;
      line-height: 1.5;
      color: #d4d4d4;
      background: #14161a;
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
      scroll-behavior: auto;

      &:focus-visible {
        outline: 2px solid var(--tt-accent);
        outline-offset: 2px;
      }
    }

    &__line {
      margin: 0;
      white-space: pre-wrap;
      overflow-wrap: anywhere;

      &--ok {
        color: #6fd391;
      }

      &--warn {
        color: #f0b04a;
      }

      &--error {
        color: #ff7b72;
      }

      &--dim {
        color: #8b949e;
      }
    }

    &__empty {
      color: #d4d4d4;
    }

    &__footer {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      gap: var(--tt-space-2);
      font-size: 12px;
      color: var(--tt-text-3);
    }
  }
</style>
