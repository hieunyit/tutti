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
   * Last step: what will be created, plus the optional name. The wizard's
   * primary button creates the replication.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import i18nAddReplication from '../i18nAddReplication';
  import ReplicationSinceField from '../ReplicationSinceField.vue';
  import ReplicationRescanField from '../ReplicationRescanField.vue';
  import ReplicationPoliciesField from '../ReplicationPoliciesField.vue';
  import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
  import { ReplicationRescanHelper } from '@/utils/helpers/ReplicationRescanHelper';
  import i18nReplicationSince from '@/components/tutti/replications/i18nReplicationSince';
  import { mergeMessages } from '@/utils/helpers/mergeMessages';
  import { ReplicationSinceHelper } from '@/utils/helpers/ReplicationSinceHelper';
  import ReplicationNameField from '@/components/tutti/replications/ReplicationNameField/ReplicationNameField.vue';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
  import { DeletePropagationMode } from '@/utils/types/tuttiCore';

  const { t, locale } = useI18n({
    messages: mergeMessages(
      i18nAddReplication,
      i18nReplicationSince,
      i18nReplicationRescan,
    ),
  });
  const {
    selectedFromStorage,
    selectedToStorage,
    selectedBuckets,
    isForAllBuckets,
    name,
    since,
    rescanMinutes,
    deleteMode,
    deleteIncludePrefixes,
    deleteExcludePrefixes,
    deleteDelaySeconds,
    includePatterns,
    excludePatterns,
  } = storeToRefs(useTuttiAddReplicationStore());

  const buckets = computed(() => [...selectedBuckets.value].sort());
  const deletePolicyText = computed(() => {
    const delay =
      deleteDelaySeconds.value > 0
        ? ` (${deleteDelaySeconds.value}s delay)`
        : '';

    if (deleteMode.value === DeletePropagationMode.SYNC) {
      return `Sync${delay}`;
    }

    if (deleteMode.value === DeletePropagationMode.MARKER_ONLY) {
      return `Marker-Only${delay}`;
    }

    return 'Ignore (do not sync deletes)';
  });
  const patternsText = computed(() => {
    const inc = includePatterns.value.length;
    const exc = excludePatterns.value.length;

    if (inc === 0 && exc === 0) {
      return 'All objects';
    }

    return `${inc} include, ${exc} exclude`;
  });
  const objectsText = computed(() => {
    const moment =
      ReplicationSinceHelper.getProblem(since.value) === null
        ? ReplicationSinceHelper.momentOf(since.value, Date.now())
        : null;

    return moment === null
      ? t('reviewObjectsAll')
      : t('reviewObjectsSince', {
          date: ReplicationSinceHelper.formatMoment(
            moment,
            String(locale.value),
          ),
        });
  });
  const rescanText = computed(() =>
    rescanMinutes.value > 0
      ? t('reviewRescanEvery', {
          interval: ReplicationRescanHelper.formatSeconds(
            rescanMinutes.value * 60,
            t('unitMin'),
            t('unitHour'),
          ),
        })
      : t('reviewRescanOff'),
  );
</script>

<template>
  <div class="review">
    <dl class="review__list">
      <div class="review__row">
        <dt>{{ t('reviewSource') }}</dt>
        <dd>{{ selectedFromStorage?.name }}</dd>
      </div>
      <div class="review__row">
        <dt>{{ t('reviewDestination') }}</dt>
        <dd>{{ selectedToStorage?.name }}</dd>
      </div>
      <div class="review__row">
        <dt>{{ t('reviewBuckets') }}</dt>
        <dd v-if="isForAllBuckets">{{ t('reviewAll') }}</dd>
        <dd v-else>
          <ul class="review__buckets">
            <li
              v-for="bucket in buckets"
              :key="bucket"
            >
              {{ bucket }}
            </li>
          </ul>
        </dd>
      </div>
      <div class="review__row">
        <dt>{{ t('reviewObjects') }}</dt>
        <dd>{{ objectsText }}</dd>
      </div>
      <div class="review__row">
        <dt>{{ t('reviewRescan') }}</dt>
        <dd>{{ rescanText }}</dd>
      </div>
      <div class="review__row">
        <dt>Delete policy</dt>
        <dd>{{ deletePolicyText }}</dd>
      </div>
      <div class="review__row">
        <dt>Pattern filters</dt>
        <dd>{{ patternsText }}</dd>
      </div>
    </dl>

    <ReplicationSinceField v-model="since" />
    <ReplicationRescanField v-model="rescanMinutes" />
    <ReplicationNameField v-model="name" />
    <ReplicationPoliciesField
      v-model:delete-mode="deleteMode"
      v-model:delete-delay-seconds="deleteDelaySeconds"
      v-model:delete-include-prefixes="deleteIncludePrefixes"
      v-model:delete-exclude-prefixes="deleteExcludePrefixes"
      v-model:include-patterns="includePatterns"
      v-model:exclude-patterns="excludePatterns"
    />

    <p class="review__note">{{ t('reviewNote') }}</p>
  </div>
</template>

<style scoped lang="scss">
  .review {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);
    max-width: 560px;

    &__list {
      display: flex;
      flex-direction: column;
      margin: 0;
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
    }

    &__row {
      display: grid;
      grid-template-columns: 120px minmax(0, 1fr);
      gap: var(--tt-space-3);
      padding: var(--tt-space-3) var(--tt-space-4);

      & + & {
        border-top: 1px solid var(--tt-border);
      }

      dt {
        color: var(--tt-text-2);
      }

      dd {
        margin: 0;
        font-weight: var(--tt-weight-strong);
        overflow-wrap: anywhere;
      }
    }

    &__buckets {
      max-height: 160px;
      margin: 0;
      padding: 0;
      list-style: none;
      overflow-y: auto;
    }

    &__note {
      margin: 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }
  }
</style>
