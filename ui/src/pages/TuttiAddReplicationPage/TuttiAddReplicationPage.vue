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
   * Add replication in three steps: Storages (source, destination), Buckets
   * (all or a selection) and Review (optional name, then one primary button).
   * The draft survives leaving the page; a failed create keeps it and says
   * what the server answered.
   */
  import { computed, onBeforeMount, onUnmounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import {
    Callout,
    clearWizardDraft,
    WizardShell,
    type WizardStep,
  } from '@/components/kit';
  import TuttiWizardPage from '@/components/tutti/common/TuttiWizardPage/TuttiWizardPage.vue';
  import StoragesStep from '@/components/tutti/add-replication/StoragesStep.vue';
  import BucketsFormStep from '@/components/tutti/add-replication/BucketsFormStep/BucketsFormStep.vue';
  import AddReplicationSummary from '@/components/tutti/add-replication/AddReplicationSummary/AddReplicationSummary.vue';
  import i18nAddReplication from '@/components/tutti/add-replication/i18nAddReplication';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
  import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';
  import {
    ReplicationSinceHelper,
    type SinceChoice,
  } from '@/utils/helpers/ReplicationSinceHelper';
  import { useStoreDraft } from '@/utils/composables/useStoreDraft';

  const DRAFT_KEY = 'add-replication';

  interface ReplicationDraft {
    from: string | null;
    to: string | null;
    buckets: string[];
    all: boolean;
    name: string;
    since?: SinceChoice;
  }

  const { t } = useI18n({ messages: i18nAddReplication });
  const store = useTuttiAddReplicationStore();
  const {
    isLoading,
    hasError,
    hasEnoughStorages,
    storages,
    selectedFromStorage,
    selectedToStorage,
    selectedBuckets,
    isForAllBuckets,
    name,
    since,
    isSubmitting,
    bucketsRequestOptions,
    createError,
  } = storeToRefs(store);

  const ready = computed(
    () => !isLoading.value && !hasError.value && hasEnoughStorages.value,
  );
  const state = computed(() => {
    if (isLoading.value) return 'loading';

    return hasError.value || !hasEnoughStorages.value ? 'error' : 'ready';
  });

  useStoreDraft<ReplicationDraft>({
    draftKey: DRAFT_KEY,
    initial: () => ({
      from: null,
      to: null,
      buckets: [],
      all: false,
      name: '',
      since: ReplicationSinceHelper.defaultChoice(),
    }),
    snapshot: () => ({
      from: selectedFromStorage.value?.name ?? null,
      to: selectedToStorage.value?.name ?? null,
      buckets: [...selectedBuckets.value],
      all: isForAllBuckets.value,
      name: name.value,
      since: { ...since.value },
    }),
    apply: (draft) => {
      const from = storages.value.find((s) => s.name === draft.from);
      const to = storages.value.find((s) => s.name === draft.to);

      if (from) selectedFromStorage.value = from;

      if (to && to.name !== from?.name) selectedToStorage.value = to;

      isForAllBuckets.value = draft.all;
      name.value = draft.name ?? '';
      since.value = draft.since
        ? { ...ReplicationSinceHelper.defaultChoice(), ...draft.since }
        : ReplicationSinceHelper.defaultChoice();

      if (from && to && draft.buckets.length) {
        // marks the bucket list as requested, so opening step 2 keeps the choice
        bucketsRequestOptions.value = {
          fromStorage: from.name,
          toStorage: to.name,
          showReplicated: true,
        };
        selectedBuckets.value = draft.buckets;
      }
    },
    ready,
  });

  const steps = computed<WizardStep[]>(() => [
    {
      key: 'storages',
      label: t('stepStorages'),
      valid:
        selectedFromStorage.value !== null &&
        selectedToStorage.value !== null &&
        selectedToStorage.value.name !== selectedFromStorage.value.name,
    },
    {
      key: 'buckets',
      label: t('stepBuckets'),
      valid: isForAllBuckets.value || selectedBuckets.value.length > 0,
    },
    {
      key: 'review',
      label: t('stepReview'),
      valid:
        ReplicationNameHelper.getProblem(name.value) === null &&
        ReplicationSinceHelper.getProblem(since.value) === null,
    },
  ]);

  async function finish() {
    const created = await store.createReplication();

    if (created) clearWizardDraft(DRAFT_KEY);
  }

  function startOver() {
    store.$reset();
    store.initAddReplicationPage();
  }

  onBeforeMount(() => store.initAddReplicationPage());
  onUnmounted(() => store.$reset());
</script>

<template>
  <TuttiWizardPage
    :title="t('title')"
    :state="state"
    :error-text="hasError ? t('loadError') : t('notEnoughStorages')"
    back-to="replication"
    @retry="store.initAddReplicationPage()"
  >
    <Callout
      v-if="createError"
      tone="danger"
      class="add-replication__error"
      :title="t('createFailedTitle')"
    >
      {{ createError }}<br />{{ t('createFailedHint') }}
    </Callout>
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('create')"
      :busy="isSubmitting"
      @finish="finish"
      @reset="startOver"
    >
      <template #storages>
        <StoragesStep />
      </template>
      <template #buckets>
        <BucketsFormStep />
      </template>
      <template #review>
        <AddReplicationSummary />
      </template>
    </WizardShell>
  </TuttiWizardPage>
</template>

<style scoped lang="scss">
  .add-replication__error {
    margin-bottom: var(--tt-space-4);
  }
</style>
