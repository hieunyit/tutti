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
  import { computed, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRouter } from 'vue-router';
  import {
    clearWizardDraft,
    WizardShell,
    type WizardStep,
  } from '@/components/kit';
  import ChorusWizardPage from '@/components/chorus/common/ChorusWizardPage/ChorusWizardPage.vue';
  import UserSelection from '@/components/chorus/add-routing-policies/UserSelection/UserSelection.vue';
  import StorageSelection from '@/components/chorus/add-routing-policies/StorageSelection/StorageSelection.vue';
  import BucketSelection from '@/components/chorus/add-routing-policies/BucketSelection/BucketSelection.vue';
  import StatusSelection from '@/components/chorus/add-routing-policies/StatusSelection/StatusSelection.vue';
  import AddRoutingPolicySummary from '@/components/chorus/add-routing-policies/AddRoutingPolicySummary/AddRoutingPolicySummary.vue';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
  import { useTuttiAddRoutingPolicyStore } from '@/stores/tuttiAddRoutingPolicyStore';
  import { useStoreDraft } from '@/utils/composables/useStoreDraft';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { RouteName } from '@/utils/types/router';

  const DRAFT_KEY = 'add-routing-policy';

  interface RoutingDraft {
    user: string | null;
    storage: string | null;
    bucket: string | null;
    all: boolean;
    blocked: boolean;
  }

  const { t } = useI18n({ messages: i18nReplicationHub });
  const router = useRouter();
  const store = useTuttiAddRoutingPolicyStore();
  const {
    isLoading,
    hasError,
    storages,
    selectedUser,
    selectedToStorage,
    bucketName,
    isForAllBuckets,
    isBlocked,
    validator,
  } = storeToRefs(store);
  const { createNotification } = useTuttiNotification();
  const isSubmitting = ref(false);

  const ready = computed(() => !isLoading.value && !hasError.value);

  useStoreDraft<RoutingDraft>({
    draftKey: DRAFT_KEY,
    initial: () => ({
      user: null,
      storage: null,
      bucket: null,
      all: false,
      blocked: false,
    }),
    snapshot: () => ({
      user: selectedUser.value,
      storage: selectedToStorage.value?.name ?? null,
      bucket: bucketName.value,
      all: isForAllBuckets.value,
      blocked: isBlocked.value,
    }),
    apply: (draft) => {
      if (draft.user && store.users.includes(draft.user))
        selectedUser.value = draft.user;

      selectedToStorage.value =
        storages.value.find((s) => s.name === draft.storage) ?? null;
      bucketName.value = draft.bucket;
      isForAllBuckets.value = draft.all;
      isBlocked.value = draft.blocked;
    },
    ready,
  });

  const steps = computed<WizardStep[]>(() => [
    { key: 'user', label: t('stepUser'), valid: !!selectedUser.value },
    { key: 'storage', label: t('stepStorage'), valid: true },
    {
      key: 'bucket',
      label: t('stepBucket'),
      valid: !validator.value.bucketName.$invalid,
    },
    { key: 'status', label: t('stepStatus'), valid: true },
    {
      key: 'confirm',
      label: t('stepConfirm'),
      valid: !validator.value.$invalid,
    },
  ]);

  async function finish() {
    isSubmitting.value = true;

    try {
      await store.addRoutingPolicy();
      createNotification({
        type: 'success',
        title: t('createdPolicy'),
        duration: 4000,
      });
      clearWizardDraft(DRAFT_KEY);
      router.push({
        name: RouteName.CHORUS_REPLICATION,
        query: { tab: 'routing' },
      });
    } catch (error: unknown) {
      createNotification({
        type: 'error',
        title: t('createFailed'),
        content: error instanceof Error ? error.message : undefined,
      });
    } finally {
      isSubmitting.value = false;
    }
  }

  function startOver() {
    store.$reset();
    store.initAddRoutingPolicyPage();
  }

  onBeforeMount(() => {
    store.initAddRoutingPolicyPage();
  });

  onUnmounted(() => {
    store.$reset();
  });
</script>

<template>
  <ChorusWizardPage
    :title="t('addRoutingTitle')"
    :description="t('addRoutingDescription')"
    :state="isLoading ? 'loading' : hasError ? 'error' : 'ready'"
    :error-text="t('wizardLoadError')"
    back-tab="routing"
    @retry="store.initAddRoutingPolicyPage()"
  >
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('finishRoutingPolicy')"
      :busy="isSubmitting"
      @finish="finish"
      @reset="startOver"
    >
      <template #user>
        <UserSelection />
      </template>
      <template #storage>
        <StorageSelection />
      </template>
      <template #bucket>
        <BucketSelection />
      </template>
      <template #status>
        <StatusSelection />
      </template>
      <template #confirm>
        <AddRoutingPolicySummary />
      </template>
    </WizardShell>
  </ChorusWizardPage>
</template>
