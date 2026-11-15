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
   * Create a diff report in two steps: source and destination, then the
   * checks (the defaults suit most). The entries survive leaving the page; a
   * failed create keeps them and says what to do next.
   */
  import { computed, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRouter } from 'vue-router';
  import {
    Callout,
    clearWizardDraft,
    WizardShell,
    type WizardStep,
  } from '@/components/kit';
  import TuttiWizardPage from '@/components/tutti/common/TuttiWizardPage/TuttiWizardPage.vue';
  import AddDiffReportSummary from '@/components/tutti/add-diff-report/AddDiffReportSummary.vue';
  import ChecksStep from '@/components/tutti/add-diff-report/ChecksStep.vue';
  import LocationsStep from '@/components/tutti/add-diff-report/LocationsStep.vue';
  import i18nAddDiffReport from '@/components/tutti/add-diff-report/i18nAddDiffReport';
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

  const { t } = useI18n({ messages: i18nAddDiffReport });
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
  const submitError = ref('');

  const ready = computed(
    () => !isLoading.value && !hasError.value && hasEnoughStorages.value,
  );
  const state = computed(() => {
    if (isLoading.value) return 'loading';

    return hasError.value || !hasEnoughStorages.value ? 'error' : 'ready';
  });

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
      key: 'locations',
      label: t('stepLocations'),
      valid:
        !validator.value.fromStorage.$invalid &&
        !validator.value.fromBucketName.$invalid &&
        !validator.value.toStorage.$invalid &&
        !validator.value.toBucketName.$invalid,
    },
    {
      key: 'checks',
      label: t('stepChecks'),
      valid: !validator.value.$invalid,
    },
  ]);

  async function finish() {
    submitError.value = '';

    try {
      await store.createDiffReport();
      createNotification({
        type: 'success',
        title: t('created'),
        duration: 4000,
      });
      clearWizardDraft(DRAFT_KEY);
      router.push({
        name: RouteName.TUTTI_REPLICATION,
        query: { tab: 'diff' },
      });
    } catch (error: unknown) {
      submitError.value =
        error instanceof Error && error.message
          ? error.message
          : t('unknownError');
    }
  }

  function startOver() {
    store.$reset();
    store.initAddDiffReportPage();
  }

  onBeforeMount(() => store.initAddDiffReportPage());
  onUnmounted(() => store.$reset());
</script>

<template>
  <TuttiWizardPage
    :title="t('title')"
    :state="state"
    :error-text="hasError ? t('loadError') : t('notEnoughStorages')"
    back-to="diff"
    @retry="store.initAddDiffReportPage()"
  >
    <Callout
      v-if="submitError"
      tone="danger"
      class="add-diff__error"
      :title="t('createFailed')"
    >
      {{ submitError }} {{ t('createFailedHint') }}
    </Callout>
    <WizardShell
      :steps="steps"
      :draft-key="DRAFT_KEY"
      :finish-label="t('create')"
      :busy="isSubmitting"
      @finish="finish"
      @reset="startOver"
    >
      <template #locations>
        <LocationsStep />
      </template>
      <template #checks>
        <ChecksStep />
      </template>
      <template #summary>
        <AddDiffReportSummary />
      </template>
    </WizardShell>
  </TuttiWizardPage>
</template>

<style scoped lang="scss">
  .add-diff__error {
    margin-bottom: var(--tt-space-4);
  }
</style>
