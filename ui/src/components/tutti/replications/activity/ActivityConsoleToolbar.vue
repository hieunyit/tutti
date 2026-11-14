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
   * Controls of the activity console: live polling and auto-scroll switches,
   * level and text filter, clear / copy / download. All controls have visible
   * labels; the buttons add a tooltip text through `title`.
   */
  import { NButton, NSwitch } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nReplicationActivity from './i18nReplicationActivity';
  import { SearchInput, SegmentedFilter } from '@/components/kit';
  import type { ActivityLevelFilter } from '@/utils/helpers/ActivityLogHelper';

  defineProps<{
    live: boolean;
    autoScroll: boolean;
    level: ActivityLevelFilter;
    text: string;
    hasLines: boolean;
    hasVisible: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'update:live', value: boolean): void;
    (e: 'update:autoScroll', value: boolean): void;
    (e: 'update:level', value: ActivityLevelFilter): void;
    (e: 'update:text', value: string): void;
    (e: 'clear'): void;
    (e: 'copy'): void;
    (e: 'download'): void;
  }>();

  const { t } = useI18n({ messages: i18nReplicationActivity });
</script>

<template>
  <div class="activity-toolbar">
    <label
      class="activity-toolbar__switch"
      :title="t('liveHint')"
    >
      <NSwitch
        size="small"
        :value="live"
        :aria-label="t('live')"
        @update:value="emit('update:live', $event)"
      />
      <span>{{ t('live') }}</span>
    </label>
    <label
      class="activity-toolbar__switch"
      :title="t('autoScrollHint')"
    >
      <NSwitch
        size="small"
        :value="autoScroll"
        :aria-label="t('autoScroll')"
        @update:value="emit('update:autoScroll', $event)"
      />
      <span>{{ t('autoScroll') }}</span>
    </label>
    <SegmentedFilter
      :model-value="level"
      :label="t('level')"
      :options="[
        { value: 'all', label: t('levelAll') },
        { value: 'info', label: t('levelInfo') },
        { value: 'warn', label: t('levelWarn') },
        { value: 'error', label: t('levelError') },
      ]"
      @update:model-value="emit('update:level', $event as ActivityLevelFilter)"
    />
    <SearchInput
      class="activity-toolbar__search"
      :model-value="text"
      :label="t('searchLabel')"
      :placeholder="t('searchPlaceholder')"
      @update:model-value="emit('update:text', $event)"
    />
    <div class="activity-toolbar__buttons">
      <NButton
        size="small"
        :disabled="!hasLines"
        :title="t('clearHint')"
        @click="emit('clear')"
        >{{ t('clear') }}</NButton
      >
      <NButton
        size="small"
        :disabled="!hasVisible"
        :title="t('copyHint')"
        @click="emit('copy')"
        >{{ t('copy') }}</NButton
      >
      <NButton
        size="small"
        :disabled="!hasLines"
        :title="t('downloadHint')"
        @click="emit('download')"
        >{{ t('download') }}</NButton
      >
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .activity-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--tt-space-3);

    &__switch {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      font-size: 13px;
      cursor: pointer;
    }

    &__search {
      flex: 1 1 200px;
      min-width: 160px;
    }

    &__buttons {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }
  }
</style>
