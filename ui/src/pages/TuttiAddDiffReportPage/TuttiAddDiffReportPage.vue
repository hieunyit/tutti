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
  import { useRouter } from 'vue-router';
  import {
    clearWizardDraft,
    WizardShell,
    type WizardStep,
  } from '@/components/kit';
  import TuttiWizardPage from '@/components/tutti/common/TuttiWizardPage/TuttiWizardPage.vue';
  import FromStep from '@/components/tutti/add-diff-report/FromStep/FromStep.vue';
  import ToStep from '@/components/tutti/add-diff-report/ToStep/ToStep.vue';
  import UserStep from '@/components/tutti/add-diff-report/UserStep/UserStep.vue';
  import AdditionalSettingsStep from '@/components/tutti/add-diff-report/AdditionalSettingsStep/AdditionalSettingsStep.vue';
  import AddDiffReportSummary from '@/components/tutti/add-diff-report/AddDiffReportSummary/AddDiffReportSummary.vue';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
  import { useTuttiAddDiffReportStore } from '@/stores/tuttiAddDiffReportStore';
  import { useStoreDraft } from '@/utils/composables/useStoreDraft';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { RouteName } from '@/utils/types/router';

  const DRAFT_KEY = 'add-diff-report';

  interface DiffDraft {
    from: string | null;
    fromBucket: string;
    to: string | null;
    toBucket: string;
    user: string | null;
    lastVersions: boolean;
    ignoreEtags: boolean;
    ignoreSizes: boolean;
  }

  const { t } = useI18n({ messages: i18nReplicationHub });
  const router = useRouter();
  const store = useTuttiAddDiffReportStore();
  const {
    isLoading,
    hasError,
    hasEnoughStorages,
    storages,
    fromStorage,
    fromBucketName,
    toStorage,
    toBucketName,
    selectedUser,
    checkOnlyLastVersions,
    ignoreEtags,
    ignoreSizes,
    isSubmitting,
    validator,
  } = storeToRefs(store);
  const { createNotification } = useTuttiNotification();

  const ready = computed(
    () => !isLoading.value && !hasError.value && hasEnoughStorages.value,
  );

  useStoreDraft<DiffDraft>({
    draftKey: DRAFT_KEY,
    initial: () => ({
      from: null,
      fromBucket: '',
      to: null,
      toBucket: '',
      user: null,
      lastVersions: true,
      ignoreEtags: false,
      ignoreSizes: false,
    }),
    snapshot: () => ({
      from: fromStorage.value?.name ?? null,
      fromBucket: fromBucketName.value,
      to: toStorage.value?.name ?? null,
      toBucket: toBucketName.value,
      user: selectedUser.value,
      lastVersions: checkOnlyLastVersions.value,
      ignoreEtags: ignoreEtags.value,
      ignoreSizes: ignoreSizes.value,
    }),
    apply: (draft) => {
      const from = storages.value.find((s) => s.name === draft.from);
      const to = storages.value.find((s) => s.name === draft.to);

      if (from) fromStorage.value = from;

      if (to && to.name !== from?.name) toStorage.value = to;

      fromBucketName.value = draft.fromBucket;
      toBucketName.value = draft.toBucket;
      checkOnlyLastVersions.value = draft.lastVersions;
      ignoreEtags.value = draft.ignoreEtags;
      ignoreSizes.value = draft.ignoreSizes;

      if (draft.user) selectedUser.value = draft.user;
    },
    ready,
  });

  const steps = computed<WizardStep[]>(() => [
    {
      key: 'from',
      label: t('stepFrom'),
      valid:
        !validator.value.fromStorage.$invalid &&
        !validator.value.fromBucketName.$invalid,
    },
    {
      key: 'to',
      label: t('stepTo'),
      valid:
        !validator.value.toStorage.$invalid &&
        !validator.value.toBucketName.$invalid,
    },
    {
      key: 'user',
      label: t('stepUser'),
      valid: !validator.value.selectedUser.$invalid,
    },
    { key: 'options', label: t('stepOptions'), valid: true },
    {
      key: 'confirm',
      label: t('stepConfirm'),
      valid: !validator.value.$invalid,
    },
  ]);

  async function finish() {
    try {
      await store.createDiffReport();
      createNotification({
        type: 'success',
        title: t('createdReport'),
        duration: 4000,
      });
      clearWizardDraft(DRAFT_KEY);
      router.push({
        name: RouteName.TUTTI_REPLICATION,
        query: { tab: 'diff' },
      });
    } catch (error: unknown) {
      createNotification({
        type: 'error',
        title: t('createFailed'),
        content: error instanceof Error ? error.message : undefined,
      });
    }
  }

  function startOver() {
    store.$reset();
    store.initAddDiffReportPage();
  }

  onBeforeMount(() => {
    store.initAddDiffReportPage();
  });

  onUnmounted(() => {
    store.$reset();
  });
</script>

<template>
  <TuttiWizardPage
    :title="t('addDiffTitle')"
    :description="t('addDiffDescription')"
    :state="
      isLoading ? 'loading' : hasError || !hasEnoughStorages ? 'error' : 'ready'
    "
    :error-text="hasError ? t('wizardLoadError') : t('notEnoughStorages')"
    back-tab="diff"
    @retry="store.initAddDiffReportPage()"
  >
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('finishDiffReport')"
      :busy="isSubmitting"
      @finish="finish"
      @reset="startOver"
    >
      <template #from>
        <FromStep />
      </template>
      <template #to>
        <ToStep />
      </template>
      <template #user>
        <UserStep />
      </template>
      <template #options>
        <AdditionalSettingsStep />
      </template>
      <template #confirm>
        <AddDiffReportSummary />
      </template>
    </WizardShell>
  </TuttiWizardPage>
</template>
