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
   * Primary text of a replication row: its name, or the auto label for an
   * unnamed one, with from → to / bucket underneath when it has a name. A small
   * edit control renames inline: Enter saves, Escape cancels, a failed save
   * keeps the edit open and shows the error.
   */
  import { computed, nextTick, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput } from 'naive-ui';
  import type { RouteLocationRaw } from 'vue-router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import i18nReplicationName from '@/components/tutti/replications/i18nReplicationName';
  import i18nReplicationSince from '@/components/tutti/replications/i18nReplicationSince';
  import { mergeMessages } from '@/utils/helpers/mergeMessages';
  import { ReplicationSinceHelper } from '@/utils/helpers/ReplicationSinceHelper';
  import {
    REPLICATION_NAME_MAX_LENGTH,
    ReplicationNameHelper,
  } from '@/utils/helpers/ReplicationNameHelper';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';

  const props = defineProps<{
    replication: TuttiReplication;
    /** persists the new (normalized) name; rejects when the server refuses it */
    rename: (name: string) => Promise<void>;
    /** when set the name is a link to the replication detail page */
    detailTo?: RouteLocationRaw;
  }>();

  const { t, locale } = useI18n({
    messages: mergeMessages(i18nReplicationName, i18nReplicationSince),
  });

  const isEditing = ref(false);
  const isSaving = ref(false);
  const draft = ref('');
  const saveError = ref('');
  const input = ref<InstanceType<typeof NInput> | null>(null);

  const primary = computed(() =>
    ReplicationNameHelper.getDisplayName(props.replication, t('allBuckets')),
  );
  const secondary = computed(() =>
    ReplicationNameHelper.getSecondaryLabel(props.replication, t('allBuckets')),
  );
  const sinceText = computed(() => {
    const date = ReplicationSinceHelper.parseStored(props.replication.since);

    return date
      ? t('listSince', {
          date: ReplicationSinceHelper.formatDay(date, String(locale.value)),
        })
      : '';
  });
  const problem = computed(() => ReplicationNameHelper.getProblem(draft.value));
  const error = computed(() => {
    if (saveError.value) return saveError.value;

    if (problem.value === 'tooLong') return t('nameTooLong');

    if (problem.value === 'controlChars') return t('nameControlChars');

    return '';
  });

  async function startEdit() {
    draft.value = props.replication.name ?? '';
    saveError.value = '';
    isEditing.value = true;
    await nextTick();
    input.value?.focus();
  }

  function cancel() {
    isEditing.value = false;
    saveError.value = '';
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      save();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      cancel();
    }
  }

  async function save() {
    if (isSaving.value || problem.value) return;

    const name = ReplicationNameHelper.normalize(draft.value);

    if (name === ReplicationNameHelper.normalize(props.replication.name)) {
      cancel();

      return;
    }

    isSaving.value = true;
    saveError.value = '';

    try {
      await props.rename(name);
      isEditing.value = false;
    } catch {
      saveError.value = t('renameFailed');
    } finally {
      isSaving.value = false;
    }
  }
</script>

<template>
  <div class="replication-name-cell">
    <div
      v-if="isEditing"
      class="replication-name-cell__edit"
    >
      <NInput
        ref="input"
        v-model:value="draft"
        size="small"
        :maxlength="REPLICATION_NAME_MAX_LENGTH"
        show-count
        :disabled="isSaving"
        :status="error ? 'error' : undefined"
        :input-props="{
          'aria-label': t('renameLabel'),
          'aria-invalid': error ? 'true' : undefined,
          autocomplete: 'off',
        }"
        @keydown="onKeydown"
        @update:value="saveError = ''"
      />
      <span
        class="replication-name-cell__help"
        :class="{ 'replication-name-cell__help--error': error }"
        :role="error ? 'alert' : undefined"
        >{{ error || t('renameHint') }}</span
      >
    </div>
    <template v-else>
      <div class="replication-name-cell__row">
        <RouterLink
          v-if="detailTo"
          class="replication-name-cell__primary replication-name-cell__link"
          :to="detailTo"
          :title="primary"
          >{{ primary }}</RouterLink
        >
        <span
          v-else
          class="replication-name-cell__primary"
          :title="primary"
          >{{ primary }}</span
        >
        <WriteGate>
          <NButton
            class="replication-name-cell__rename"
            quaternary
            size="tiny"
            :aria-label="`${t('renameAction')}: ${primary}`"
            @click="startEdit"
            >{{ t('renameAction') }}</NButton
          >
        </WriteGate>
      </div>
      <span
        v-if="secondary"
        class="replication-name-cell__secondary"
        :title="secondary"
        >{{ secondary }}</span
      >
      <span
        v-if="sinceText"
        class="replication-name-cell__secondary replication-name-cell__since"
        :title="sinceText"
        >{{ sinceText }}</span
      >
    </template>
  </div>
</template>

<style lang="scss" scoped>
  .replication-name-cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    max-width: 100%;

    /* on a pointer device the rename control appears with the row hover or focus */
    @media (hover: hover) {
      &__rename {
        opacity: 0;
        transition: opacity var(--tt-duration) var(--tt-ease);
      }

      &:hover &__rename,
      &:focus-within &__rename,
      :global(.n-data-table-tr:hover) &__rename {
        opacity: 1;
      }
    }

    &__row,
    &__edit {
      display: flex;
      min-width: 0;
      gap: var(--tt-space-2);
    }

    &__row {
      align-items: center;
    }

    &__edit {
      flex-direction: column;
    }

    &__primary,
    &__secondary {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__primary {
      font-weight: 600;
    }

    &__link {
      color: var(--tt-accent);
      text-decoration: none;

      &:hover {
        color: var(--tt-accent-hover);
        text-decoration: underline;
      }

      &:focus-visible {
        outline: 2px solid var(--tt-accent);
        outline-offset: 2px;
      }
    }

    &__secondary,
    &__help {
      font-size: 0.8125rem;
      opacity: 0.75;
    }

    &__help--error {
      opacity: 1;
      color: var(--tt-danger, #d03050);
    }
  }
</style>
