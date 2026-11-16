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
  import i18nReplicationSince from '@/components/tutti/replications/i18nReplicationSince';
  import { mergeMessages } from '@/utils/helpers/mergeMessages';
  import { ReplicationSinceHelper } from '@/utils/helpers/ReplicationSinceHelper';
  import ReplicationNameField from '@/components/tutti/replications/ReplicationNameField/ReplicationNameField.vue';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';

  const { t, locale } = useI18n({
    messages: mergeMessages(i18nAddReplication, i18nReplicationSince),
  });
  const {
    selectedFromStorage,
    selectedToStorage,
    selectedBuckets,
    isForAllBuckets,
    name,
    since,
  } = storeToRefs(useTuttiAddReplicationStore());

  const buckets = computed(() => [...selectedBuckets.value].sort());
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
    </dl>

    <ReplicationSinceField v-model="since" />
    <ReplicationNameField v-model="name" />

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
