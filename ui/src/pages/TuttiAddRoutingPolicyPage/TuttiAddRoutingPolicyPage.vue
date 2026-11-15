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
   * Add a routing rule: one form (user, buckets, requests) with sensible
   * defaults. The entries survive leaving the page; a failed create keeps them
   * and says what to do next.
   */
  import { computed, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRouter } from 'vue-router';
  import { NButton } from 'naive-ui';
  import { Callout, clearWizardDraft } from '@/components/kit';
  import RoutingRuleForm from '@/components/tutti/add-routing-policies/RoutingRuleForm.vue';
  import i18nAddRoutingPolicy from '@/components/tutti/add-routing-policies/i18nAddRoutingPolicy';
  import TuttiWizardPage from '@/components/tutti/common/TuttiWizardPage/TuttiWizardPage.vue';
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

  const { t } = useI18n({ messages: i18nAddRoutingPolicy });
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
  const submitError = ref('');

  const ready = computed(() => !isLoading.value && !hasError.value);

  useStoreDraft<RoutingDraft>({
    draftKey: DRAFT_KEY,
    initial: () => ({
      user: null,
      storage: null,
      bucket: null,
      all: true,
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
      if (draft.user) selectedUser.value = draft.user;

      // the only storage is the obvious choice
      selectedToStorage.value =
        storages.value.find((s) => s.name === draft.storage) ??
        (storages.value.length === 1 ? (storages.value[0] ?? null) : null);
      bucketName.value = draft.bucket;
      isForAllBuckets.value = draft.all;
      isBlocked.value = draft.blocked;
    },
    ready,
  });

  async function submit() {
    validator.value.$touch();

    if (validator.value.$invalid) return;

    isSubmitting.value = true;
    submitError.value = '';

    try {
      await store.addRoutingPolicy();
      createNotification({
        type: 'success',
        title: t('created'),
        duration: 4000,
      });
      clearWizardDraft(DRAFT_KEY);
      router.push({ name: RouteName.TUTTI_ROUTING_RULES });
    } catch (error: unknown) {
      submitError.value =
        error instanceof Error ? error.message : t('createErrorUnknown');
    } finally {
      isSubmitting.value = false;
    }
  }

  onBeforeMount(() => {
    store.initAddRoutingPolicyPage();
  });
  onUnmounted(() => store.$reset());
</script>

<template>
  <TuttiWizardPage
    :title="t('title')"
    :state="isLoading ? 'loading' : hasError ? 'error' : 'ready'"
    :error-text="t('loadError')"
    back-to="routing"
    @retry="store.initAddRoutingPolicyPage()"
  >
    <form
      class="add-rule"
      novalidate
      @submit.prevent="submit"
    >
      <Callout
        v-if="submitError"
        tone="danger"
        :title="t('createFailed')"
      >
        {{ submitError }} {{ t('createFailedHint') }}
      </Callout>

      <RoutingRuleForm />

      <div class="add-rule__footer">
        <RouterLink :to="{ name: RouteName.TUTTI_ROUTING_RULES }">
          <NButton
            tag="span"
            :disabled="isSubmitting"
          >
            {{ t('cancel') }}
          </NButton>
        </RouterLink>
        <NButton
          type="primary"
          attr-type="submit"
          :loading="isSubmitting"
        >
          {{ t('create') }}
        </NButton>
      </div>
    </form>
  </TuttiWizardPage>
</template>

<style scoped lang="scss">
  .add-rule {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);

    &__footer {
      display: flex;
      gap: var(--tt-space-2);
      padding-top: var(--tt-space-3);
      border-top: 1px solid var(--tt-border);
    }
  }
</style>
