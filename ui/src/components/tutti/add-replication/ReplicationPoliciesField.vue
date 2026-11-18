<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -->
<script setup lang="ts">
  import { computed } from 'vue';
  import { NSelect, NInputNumber, NDynamicTags } from 'naive-ui';
  import { DeletePropagationMode } from '@/utils/types/tuttiCore';

  const deleteMode = defineModel<DeletePropagationMode>('deleteMode', {
    required: true,
  });
  const deleteDelaySeconds = defineModel<number>('deleteDelaySeconds', {
    required: true,
  });
  const deleteIncludePrefixes = defineModel<string[]>('deleteIncludePrefixes', {
    required: true,
  });
  const deleteExcludePrefixes = defineModel<string[]>('deleteExcludePrefixes', {
    required: true,
  });
  const includePatterns = defineModel<string[]>('includePatterns', {
    required: true,
  });
  const excludePatterns = defineModel<string[]>('excludePatterns', {
    required: true,
  });

  const modeOptions = computed(() => [
    {
      label: 'Ignore (Do not sync deletes)',
      value: DeletePropagationMode.IGNORE,
    },
    {
      label: 'Sync (Execute hard deletes at destination)',
      value: DeletePropagationMode.SYNC,
    },
    {
      label: 'Marker-Only (Create delete markers at destination)',
      value: DeletePropagationMode.MARKER_ONLY,
    },
  ]);
</script>

<template>
  <div class="replication-policies">
    <!-- Delete Propagation Section -->
    <fieldset class="policy-fieldset">
      <legend class="policy-fieldset__legend">Delete Propagation Policy</legend>
      <div class="policy-fieldset__content">
        <div class="policy-field">
          <label class="policy-field__label">Deletion Sync Mode</label>
          <NSelect
            v-model:value="deleteMode"
            :options="modeOptions"
          />
          <p class="policy-field__help">
            Controls how source object deletions propagate to the destination
            replica.
          </p>
        </div>

        <div
          v-if="deleteMode !== DeletePropagationMode.IGNORE"
          class="policy-field"
        >
          <label class="policy-field__label"
            >Delay Before Delete (Seconds)</label
          >
          <NInputNumber
            v-model:value="deleteDelaySeconds"
            :min="0"
            :max="7776000"
            style="max-width: 200px"
          />
          <p class="policy-field__help">
            Grace period before deleting replicas (e.g. 86400 = 24h soft delete
            protection).
          </p>
        </div>

        <div
          v-if="deleteMode !== DeletePropagationMode.IGNORE"
          class="policy-grid"
        >
          <div class="policy-field">
            <label class="policy-field__label"
              >Include Prefixes (Optional)</label
            >
            <NDynamicTags v-model:value="deleteIncludePrefixes" />
            <p class="policy-field__help">
              Only delete objects under these prefixes.
            </p>
          </div>
          <div class="policy-field">
            <label class="policy-field__label"
              >Exclude Prefixes (Optional)</label
            >
            <NDynamicTags v-model:value="deleteExcludePrefixes" />
            <p class="policy-field__help">
              Never delete objects under these prefixes.
            </p>
          </div>
        </div>
      </div>
    </fieldset>

    <!-- Advanced Pattern Filtering Section -->
    <fieldset class="policy-fieldset">
      <legend class="policy-fieldset__legend">
        Advanced Pattern Filtering (Glob Wildcard)
      </legend>
      <div class="policy-fieldset__content">
        <div class="policy-field">
          <label class="policy-field__label">Include Patterns (Optional)</label>
          <NDynamicTags v-model:value="includePatterns" />
          <p class="policy-field__help">
            Synchronize only matching objects (e.g. <code>*.jpg</code>,
            <code>data/**</code>, <code>logs/*</code>).
          </p>
        </div>

        <div class="policy-field">
          <label class="policy-field__label">Exclude Patterns (Optional)</label>
          <NDynamicTags v-model:value="excludePatterns" />
          <p class="policy-field__help">
            Skip matching objects (e.g. <code>*.tmp</code>,
            <code>cache/**</code>, <code>drafts/*</code>). Exclude takes
            precedence.
          </p>
        </div>
      </div>
    </fieldset>
  </div>
</template>

<style scoped lang="scss">
  .replication-policies {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    max-width: 560px;
  }

  .policy-fieldset {
    margin: 0;
    padding: var(--tt-space-4);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);

    &__legend {
      padding: 0 var(--tt-space-2);
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text-1);
    }

    &__content {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-3);
    }
  }

  .policy-field {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);

    &__label {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text-2);
    }

    &__help {
      margin: 0;
      font-size: 0.75rem;
      color: var(--tt-text-3);
    }
  }

  .policy-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--tt-space-3);

    @media (max-width: 600px) {
      grid-template-columns: 1fr;
    }
  }
</style>
