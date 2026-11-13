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
  import { computed, onBeforeMount, onUnmounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import {
    clearWizardDraft,
    WizardShell,
    type WizardStep,
  } from '@/components/kit';
  import ChorusWizardPage from '@/components/chorus/common/ChorusWizardPage/ChorusWizardPage.vue';
  import FromStorageFormStep from '@/components/chorus/add-replication/FromStorageFormStep/FromStorageFormStep.vue';
  import ToStorageFormStep from '@/components/chorus/add-replication/ToStorageFormStep/ToStorageFormStep.vue';
  import BucketsFormStep from '@/components/chorus/add-replication/BucketsFormStep/BucketsFormStep.vue';
  import AddReplicationSummary from '@/components/chorus/add-replication/AddReplicationSummary/AddReplicationSummary.vue';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
  import { useStoreDraft } from '@/utils/composables/useStoreDraft';

  const DRAFT_KEY = 'add-replication';

  interface ReplicationDraft {
    from: string | null;
    to: string | null;
    user: string | null;
    buckets: string[];
    all: boolean;
  }

  const { t } = useI18n({ messages: i18nReplicationHub });
  const store = useTuttiAddReplicationStore();
  const {
    isLoading,
    hasError,
    storages,
    selectedFromStorage,
    selectedToStorage,
    selectedUser,
    selectedBuckets,
    isForAllBuckets,
    isSubmitting,
    bucketsRequestOptions,
  } = storeToRefs(store);

  const ready = computed(
    () => !isLoading.value && !hasError.value && storages.value.length > 0,
  );

  useStoreDraft<ReplicationDraft>({
    draftKey: DRAFT_KEY,
    initial: () => ({
      from: null,
      to: null,
      user: null,
      buckets: [],
      all: false,
    }),
    snapshot: () => ({
      from: selectedFromStorage.value?.name ?? null,
      to: selectedToStorage.value?.name ?? null,
      user: selectedUser.value,
      buckets: [...selectedBuckets.value],
      all: isForAllBuckets.value,
    }),
    apply: (draft) => {
      const from = storages.value.find((s) => s.name === draft.from);
      const to = storages.value.find((s) => s.name === draft.to);

      if (from) selectedFromStorage.value = from;

      if (to && to.name !== from?.name) selectedToStorage.value = to;

      if (draft.user) selectedUser.value = draft.user;

      isForAllBuckets.value = draft.all;

      if (from && to && draft.user && draft.buckets.length) {
        // marks the bucket list as requested, so opening step 3 keeps the choice
        bucketsRequestOptions.value = {
          user: draft.user,
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
      key: 'from',
      label: t('stepFrom'),
      valid: selectedFromStorage.value !== null,
    },
    {
      key: 'to',
      label: t('stepTo'),
      valid:
        selectedToStorage.value !== null &&
        selectedToStorage.value.name !== selectedFromStorage.value?.name,
    },
    {
      key: 'buckets',
      label: t('stepBuckets'),
      valid: isForAllBuckets.value || selectedBuckets.value.length > 0,
    },
    { key: 'confirm', label: t('stepConfirm'), valid: true },
  ]);

  async function finish() {
    await store.createReplication();
    clearWizardDraft(DRAFT_KEY);
  }

  function startOver() {
    store.$reset();
    store.initAddReplicationPage();
  }

  onBeforeMount(() => {
    store.initAddReplicationPage();
  });

  onUnmounted(() => {
    store.$reset();
  });
</script>

<template>
  <ChorusWizardPage
    :title="t('addReplicationTitle')"
    :description="t('addReplicationDescription')"
    :state="isLoading ? 'loading' : hasError ? 'error' : 'ready'"
    :error-text="t('wizardLoadError')"
    @retry="store.initAddReplicationPage()"
  >
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('finishReplication')"
      :busy="isSubmitting"
      @finish="finish"
      @reset="startOver"
    >
      <template #from>
        <FromStorageFormStep />
      </template>
      <template #to>
        <ToStorageFormStep />
      </template>
      <template #buckets>
        <BucketsFormStep />
      </template>
      <template #confirm>
        <AddReplicationSummary />
      </template>
    </WizardShell>
  </ChorusWizardPage>
</template>
