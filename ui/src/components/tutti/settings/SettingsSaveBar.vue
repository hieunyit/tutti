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
   * Sticky bar shown while the draft differs from the saved settings:
   * Save opens a diff summary, Cancel discards the draft.
   */
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import messages from './i18nSettings';
  import { diffSettings } from './settingsMeta';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { ConfirmDialog } from '@/components/kit';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';

  const emit = defineEmits<{
    (e: 'discard'): void;
    (e: 'invalid'): void;
  }>();

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const review = ref(false);
  const changes = computed(() =>
    review.value ? diffSettings(store.settings, store.draft) : [],
  );
  const invalidCount = computed(
    () => Object.keys(store.validationErrors).length,
  );
  const changeCount = computed(
    () => diffSettings(store.settings, store.draft).length,
  );

  /** The one status text: unsaved count while editing, else the saved version. */
  const statusText = computed(() => {
    if (store.hasChanges)
      return t('unsaved', { count: changeCount.value }, changeCount.value);

    const saved = store.settings;

    return saved?.updatedAt
      ? t('statusSaved', {
          version: saved.version ?? 0,
          when: formatDateTime(saved.updatedAt),
        })
      : t('statusNotSaved');
  });

  function openReview() {
    if (invalidCount.value) emit('invalid');
    else review.value = true;
  }

  async function confirm() {
    review.value = false;
    await store.save();
  }
</script>

<template>
  <div
    class="settings-bar"
    :class="{ 'settings-bar--dirty': store.hasChanges }"
    role="status"
    aria-live="polite"
  >
    <span
      class="settings-bar__text"
      :class="{ 'settings-bar__text--dirty': store.hasChanges }"
      >{{ statusText }}</span
    >
    <span
      v-if="store.hasChanges && invalidCount"
      class="settings-bar__invalid"
      >{{ t('validationSummary', { count: invalidCount }) }}
      <NButton
        text
        type="error"
        @click="emit('invalid')"
        >{{ t('reviewFields') }}</NButton
      ></span
    >
    <div
      v-if="store.hasChanges"
      class="settings-bar__actions"
    >
      <NButton
        :disabled="!store.hasChanges || store.isSaving"
        data-test="settings-cancel"
        @click="emit('discard')"
        >{{ t('reset') }}</NButton
      >
      <WriteGate>
        <NButton
          type="primary"
          data-test="settings-save"
          :loading="store.isSaving"
          :disabled="!store.hasChanges || store.isSaving"
          @click="openReview"
          >{{ store.isSaving ? t('saving') : t('save') }}</NButton
        >
      </WriteGate>
    </div>
  </div>

  <ConfirmDialog
    :open="review"
    :title="t('diffTitle')"
    :confirm-label="t('confirmSave')"
    :loading="store.isSaving"
    @confirm="confirm"
    @cancel="review = false"
  >
    <p>
      {{
        t('diffText', { version: (Number(store.settings?.version) || 0) + 1 })
      }}
    </p>
    <ul class="settings-bar__diff">
      <li
        v-for="change in changes"
        :key="change.key"
      >
        <strong>{{ t(change.key) }}</strong
        >:
        <code class="tt-mono">{{ change.from || '–' }}</code>
        →
        <code class="tt-mono">{{ change.to || '–' }}</code>
        <em v-if="change.restart"> ({{ t('restartRequired') }})</em>
      </li>
      <li v-if="!changes.length">{{ t('diffNoChanges') }}</li>
    </ul>
  </ConfirmDialog>
</template>

<style lang="scss" scoped>
  .settings-bar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--tt-space-3);
    margin-top: var(--tt-space-5);
    font-size: var(--tt-text-sm);
    color: var(--tt-text-3);

    // only while editing does the bar float with the actions
    &--dirty {
      position: sticky;
      bottom: 0;
      z-index: 5;
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);
      box-shadow: var(--tt-shadow-overlay);
    }

    &__text--dirty {
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text);
    }

    &__invalid {
      font-size: var(--tt-text-sm);
      color: var(--tt-danger);
    }

    &__actions {
      display: inline-flex;
      gap: var(--tt-space-2);
      margin-left: auto;
    }

    &__diff {
      margin: var(--tt-space-2) 0 0;
      padding-left: var(--tt-space-4);
      font-size: var(--tt-text-sm);
      line-height: 22px;
    }
  }
</style>
