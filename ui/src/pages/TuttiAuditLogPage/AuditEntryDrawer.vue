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
  /** Side panel with one audit entry: summary, changed fields, before/after JSON. */
  import { computed } from 'vue';
  import { NDrawer, NDrawerContent } from 'naive-ui';
  import {
    actionTone,
    changedTopLevelKeys,
    formatJsonPayload,
    useAuditI18n,
  } from './useAuditI18n';
  import { CopyValue, EMPTY_VALUE, StatusBadge } from '@/components/kit';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type { AuditLogEntry } from '@/utils/types/audit';

  const props = defineProps<{ entry: AuditLogEntry | null }>();
  const emit = defineEmits<{ (e: 'close'): void }>();

  const { t, familyLabel, actionLabel } = useAuditI18n();
  const before = computed(() => formatJsonPayload(props.entry?.beforeJson));
  const after = computed(() => formatJsonPayload(props.entry?.afterJson));
  const changed = computed(() =>
    props.entry
      ? changedTopLevelKeys(props.entry.beforeJson, props.entry.afterJson)
      : [],
  );
</script>

<template>
  <NDrawer
    :show="entry !== null"
    :width="640"
    placement="right"
    @update:show="(show: boolean) => !show && emit('close')"
  >
    <NDrawerContent
      v-if="entry"
      closable
      :title="t('drawerTitle', { id: entry.id })"
    >
      <dl class="audit-drawer__summary">
        <div>
          <dt>{{ t('colTimestamp') }}</dt>
          <dd>
            {{ formatDateTime(entry.at) }}
            <span class="tt-mono audit-drawer__muted">({{ entry.at }})</span>
          </dd>
        </div>
        <div>
          <dt>{{ t('colActor') }}</dt>
          <dd>
            {{ entry.actor || EMPTY_VALUE }}
            <CopyValue
              v-if="entry.requestId"
              :value="entry.requestId"
            />
          </dd>
        </div>
        <div>
          <dt>{{ t('colAction') }}</dt>
          <dd>
            <StatusBadge
              :tone="actionTone(entry.action)"
              :label="actionLabel(entry.action)"
            />
          </dd>
        </div>
        <div>
          <dt>{{ t('colFamily') }}</dt>
          <dd>
            {{ familyLabel(entry.family) }}
            <span class="tt-mono audit-drawer__muted">{{ entry.family }}</span>
          </dd>
        </div>
        <div>
          <dt>{{ t('colEntityKey') }}</dt>
          <dd class="tt-mono">{{ entry.entityKey || EMPTY_VALUE }}</dd>
        </div>
        <div>
          <dt>{{ t('changedFields') }}</dt>
          <dd>
            <template v-if="changed.length">
              <code
                v-for="field in changed"
                :key="field"
                class="audit-drawer__chip tt-mono"
                >{{ field }}</code
              >
            </template>
            <span
              v-else
              class="audit-drawer__muted"
              >{{ t('noChangedFields') }}</span
            >
          </dd>
        </div>
      </dl>
      <p class="audit-drawer__muted">{{ t('redactedNote') }}</p>
      <div class="audit-drawer__panes">
        <section class="audit-drawer__pane">
          <h3>{{ t('beforeSnapshot') }}</h3>
          <pre
            v-if="before"
            class="tt-mono"
            >{{ before }}</pre
          >
          <p
            v-else
            class="audit-drawer__muted"
          >
            {{ t('noBeforeSnapshot') }}
          </p>
        </section>
        <section class="audit-drawer__pane">
          <h3>{{ t('afterSnapshot') }}</h3>
          <pre
            v-if="after"
            class="tt-mono"
            >{{ after }}</pre
          >
          <p
            v-else
            class="audit-drawer__muted"
          >
            {{ t('noAfterSnapshot') }}
          </p>
        </section>
      </div>
    </NDrawerContent>
  </NDrawer>
</template>

<style lang="scss" scoped>
  .audit-drawer {
    &__summary {
      display: grid;
      gap: var(--tt-space-2);
      margin: 0 0 var(--tt-space-3);

      > div {
        display: grid;
        grid-template-columns: 120px 1fr;
        gap: var(--tt-space-2);
      }

      dt {
        color: var(--tt-text-2);
        font-size: var(--tt-text-sm);
      }

      dd {
        margin: 0;
        min-width: 0;
        overflow-wrap: anywhere;
      }
    }

    &__muted {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }

    &__chip {
      display: inline-block;
      margin: 0 var(--tt-space-1) var(--tt-space-1) 0;
      padding: 1px 6px;
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-tag);
      background-color: var(--tt-code-bg);
      font-size: var(--tt-text-xs);
    }

    &__panes {
      display: grid;
      gap: var(--tt-space-3);
    }

    &__pane {
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      overflow: hidden;

      h3 {
        margin: 0;
        padding: var(--tt-space-2) var(--tt-space-3);
        background-color: var(--tt-subtle);
        border-bottom: 1px solid var(--tt-border);
        font-size: var(--tt-text-xs);
        font-weight: var(--tt-weight-strong);
        color: var(--tt-text-2);
      }

      pre {
        margin: 0;
        padding: var(--tt-space-3);
        max-height: 320px;
        overflow: auto;
        font-size: var(--tt-text-xs);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
      }

      p {
        margin: 0;
        padding: var(--tt-space-3);
      }
    }
  }
</style>
