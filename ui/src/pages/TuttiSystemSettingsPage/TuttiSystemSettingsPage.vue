<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, nextTick, onMounted, onBeforeUnmount, ref } from 'vue';
  import { onBeforeRouteLeave } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton, NSwitch, NInput, NInputNumber, NSkeleton } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import Callout from '@/components/ui/Callout.vue';
  import ConfirmDialog from '@/components/tutti/common/ConfirmDialog.vue';
  import ErrorState from '@/components/tutti/common/ErrorState.vue';
  import TabBar from '@/components/tutti/common/TabBar.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import messages from '@/components/tutti/settings/i18nSettings';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';
  import { useAuthStore } from '@/stores/authStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { WORKER_DURATION_FIELDS } from '@/utils/helpers/SystemSettingsHelper';

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const auth = useAuthStore();
  const isDisabled = computed(
    () => store.isLoading || store.isSaving || !auth.isAdmin,
  );
  const featureKeys = [
    'tagging',
    'acl',
    'lifecycle',
    'policy',
    'preserveAclGrants',
    'versioning',
  ] as const;
  const watermarkKeys = [
    'backlogHighWatermark',
    'backlogLowWatermark',
  ] as const;
  const byteKeys = ['bandwidthLimitBytes', 'bufferBudgetBytes'] as const;
  const activeTab = ref('features');
  const sections = computed(() => [
    {
      value: 'features',
      label: t('tabFeatures'),
      title: t('featuresTitle'),
      description: t('featuresDesc'),
    },
    {
      value: 'bulk',
      label: t('tabBulk'),
      title: t('bulkTitle'),
      description: t('bulkDesc'),
    },
    {
      value: 'worker',
      label: t('tabWorker'),
      title: t('workerTitle'),
      description: t('workerDesc'),
    },
    {
      value: 'locking',
      label: t('tabLocking'),
      title: t('lockTitle'),
      description: t('lockDesc'),
    },
    {
      value: 'webhook',
      label: t('tabWebhook'),
      title: t('webhookTitle'),
      description: t('webhookDesc'),
    },
  ]);
  const currentSection = computed(
    () =>
      sections.value.find((section) => section.value === activeTab.value) ??
      sections.value[0]!,
  );
  const invalidFields = computed(() => Object.keys(store.validationErrors));
  const tabs = computed(() =>
    sections.value.map((section) => ({
      value: section.value,
      label: section.label,
      count:
        invalidFields.value.filter(
          (field) => sectionForField(field) === section.value,
        ).length || undefined,
    })),
  );

  function sectionForField(field: string) {
    if (field === 'lockOverlap') return 'locking';

    if (field === 'webhookBaseUrl') return 'webhook';

    return [...watermarkKeys, ...byteKeys].some((key) => key === field)
      ? 'bulk'
      : 'worker';
  }

  async function reviewInvalidFields() {
    const field = invalidFields.value[0];

    if (!field) return;

    activeTab.value = sectionForField(field);
    await nextTick();
    document.getElementById(`settings-${field}`)?.focus();
  }
  const showDiscard = ref(false);
  let resolveDiscard: ((confirmed: boolean) => void) | undefined;

  function finishDiscard(confirmed: boolean) {
    showDiscard.value = false;
    resolveDiscard?.(confirmed);
    resolveDiscard = undefined;
  }

  function confirmDiscard(): Promise<boolean> {
    resolveDiscard?.(false);
    showDiscard.value = true;

    return new Promise((resolve) => {
      resolveDiscard = resolve;
    });
  }

  async function handleRefresh() {
    if (isDisabled.value) return;

    if (store.hasChanges && !(await confirmDiscard())) return;

    await store.load();
  }

  async function handleReset() {
    if (await confirmDiscard()) store.resetDraft();
  }

  function beforeUnload(event: BeforeUnloadEvent) {
    if (!store.hasChanges) return;

    event.preventDefault();
    event.returnValue = '';
  }

  onBeforeRouteLeave(async () => {
    if (store.isSaving) return false;

    if (!store.hasChanges) return true;

    if (!(await confirmDiscard())) return false;

    store.resetDraft();

    return true;
  });

  onMounted(() => {
    if (!store.settings) void store.load();

    window.addEventListener('beforeunload', beforeUnload);
  });
  onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', beforeUnload);
    finishDiscard(false);
  });
</script>

<template>
  <div class="phase-d settings-page">
    <PageHeader
      :title="t('title')"
      :description="t('subtitle')"
    >
      <template #actions>
        <NButton
          :disabled="isDisabled"
          :loading="store.isLoading"
          @click="handleRefresh"
        >
          {{ t('refresh') }}
        </NButton>
      </template>
      <template
        v-if="store.settings"
        #extra
      >
        <div
          class="phase-d__toolbar"
          role="status"
          aria-live="polite"
        >
          <StatusTag
            variant="neutral"
            no-dot
            >{{ t('savedVersion') }}
            {{ store.settings.version ?? 0 }}</StatusTag
          >
          <span class="phase-d__muted"
            >{{ t('lastUpdated') }}:
            {{
              store.settings.updatedAt
                ? formatDateTime(store.settings.updatedAt)
                : t('never')
            }}</span
          >
          <StatusTag
            v-if="store.hasChanges"
            variant="warning"
            no-dot
            >{{ t('unsavedChanges') }}</StatusTag
          >
        </div>
      </template>
    </PageHeader>

    <SectionCard
      v-if="store.isLoading && !store.settings"
      :title="t('configuration')"
    >
      <div
        role="status"
        :aria-label="t('loading')"
      >
        <NSkeleton
          text
          :repeat="4"
        />
        <span class="tt-sr-only">{{ t('loading') }}</span>
      </div>
    </SectionCard>
    <ErrorState
      v-else-if="store.hasError && !store.settings"
      :title="t('requestFailed')"
      :text="t('loadFailed')"
      :action-label="t('refresh')"
      @retry="handleRefresh"
    />
    <Callout
      v-if="store.hasError && store.settings && !store.conflictError"
      tone="danger"
      :title="t('requestFailed')"
    >
      {{ store.errorMessage }}
    </Callout>
    <Callout
      v-if="store.conflictError"
      tone="warning"
      :title="t('conflictError')"
    >
      {{ t('draftRetained') }}
      <template #actions
        ><NButton
          :disabled="isDisabled"
          @click="handleRefresh"
          >{{ t('refresh') }}</NButton
        ></template
      >
    </Callout>

    <form
      v-if="store.settings"
      class="settings-page__form"
      :aria-busy="store.isLoading || store.isSaving"
      @submit.prevent="store.save()"
    >
      <TabBar
        v-model="activeTab"
        :tabs="tabs"
        :label="t('settingsSections')"
      />
      <SectionCard :title="currentSection.title">
        <template
          v-if="activeTab === 'worker'"
          #actions
        >
          <StatusTag
            variant="warning"
            no-dot
            >{{ t('restartRequired') }}</StatusTag
          >
        </template>
        <p class="phase-d__muted settings-page__description">
          {{ currentSection.description }}
        </p>
        <div
          v-show="activeTab === 'features'"
          id="tabpanel-features"
          class="phase-d__form"
          role="tabpanel"
          aria-labelledby="tab-features"
        >
          <div class="settings-page__switches">
            <div
              v-for="key in featureKeys"
              :key="key"
              class="settings-page__switch-row"
            >
              <div class="settings-page__switch-copy">
                <span
                  :id="`settings-${key}-label`"
                  class="settings-page__feature-title"
                  >{{ t(key) }}</span
                >
                <span
                  :id="`settings-${key}-hint`"
                  class="phase-d__muted"
                  >{{ t(`${key}Desc`) }}</span
                >
              </div>
              <NSwitch
                v-model:value="store.draft.features![key]"
                :disabled="isDisabled"
                :aria-labelledby="`settings-${key}-label`"
                :aria-describedby="`settings-${key}-hint`"
              />
            </div>
          </div>
        </div>
        <div
          v-show="activeTab === 'bulk'"
          id="tabpanel-bulk"
          class="phase-d__form"
          role="tabpanel"
          aria-labelledby="tab-bulk"
        >
          <div class="phase-d__grid">
            <div
              v-for="key in watermarkKeys"
              :key="key"
              class="phase-d__field"
            >
              <label
                :for="`settings-${key}`"
                class="phase-d__label"
                >{{ t(key) }}</label
              >
              <NInputNumber
                class="settings-page__number"
                v-model:value="store.draft.worker![key]"
                :disabled="isDisabled"
                :min="1"
                :max="2147483647"
                :precision="0"
                :status="store.validationErrors[key] ? 'error' : undefined"
                :input-props="{
                  id: `settings-${key}`,
                  'aria-describedby': `settings-${key}-hint`,
                  'aria-invalid': Boolean(store.validationErrors[key]),
                }"
              />
              <span
                :id="`settings-${key}-hint`"
                class="phase-d__muted"
                :class="{
                  'settings-page__error': store.validationErrors[key],
                }"
                >{{ t(store.validationErrors[key] || `${key}Hint`) }}</span
              >
            </div>
            <div
              v-for="key in byteKeys"
              :key="key"
              class="phase-d__field"
            >
              <label
                :for="`settings-${key}`"
                class="phase-d__label"
                >{{ t(key) }}</label
              >
              <NInput
                v-model:value="store.draft.worker![key]"
                :disabled="isDisabled"
                :status="store.validationErrors[key] ? 'error' : undefined"
                :input-props="{
                  id: `settings-${key}`,
                  inputmode: 'numeric',
                  'aria-describedby': `settings-${key}-hint`,
                  'aria-invalid': Boolean(store.validationErrors[key]),
                }"
              />
              <span
                :id="`settings-${key}-hint`"
                class="phase-d__muted"
                :class="{
                  'settings-page__error': store.validationErrors[key],
                }"
                >{{
                  t(
                    store.validationErrors[key] ||
                      (key === 'bandwidthLimitBytes'
                        ? 'bandwidthLimitHint'
                        : 'bufferBudgetHint'),
                  )
                }}</span
              >
            </div>
          </div>
        </div>
        <div
          v-show="activeTab === 'worker'"
          id="tabpanel-worker"
          class="phase-d__form"
          role="tabpanel"
          aria-labelledby="tab-worker"
        >
          <p
            id="settings-duration-hint"
            class="phase-d__muted settings-page__description"
          >
            {{ t('durationHint') }}
          </p>
          <div class="phase-d__grid">
            <div class="phase-d__field">
              <label
                for="settings-concurrency"
                class="phase-d__label"
                >{{ t('concurrency') }}</label
              >
              <NInputNumber
                class="settings-page__number"
                v-model:value="store.draft.worker!.concurrency"
                :disabled="isDisabled"
                :min="1"
                :max="1000"
                :precision="0"
                :status="
                  store.validationErrors.concurrency ? 'error' : undefined
                "
                :input-props="{
                  id: 'settings-concurrency',
                  'aria-describedby': 'settings-concurrency-hint',
                  'aria-invalid': Boolean(store.validationErrors.concurrency),
                }"
              />
              <span
                id="settings-concurrency-hint"
                class="phase-d__muted"
                :class="{
                  'settings-page__error': store.validationErrors.concurrency,
                }"
                >{{
                  t(store.validationErrors.concurrency || 'concurrencyHint')
                }}</span
              >
            </div>
            <div
              v-for="key in WORKER_DURATION_FIELDS"
              :key="key"
              class="phase-d__field"
            >
              <label
                :for="`settings-${key}`"
                class="phase-d__label"
                >{{ t(key) }}</label
              >
              <NInput
                v-model:value="store.draft.worker![key]"
                :disabled="isDisabled"
                placeholder="30s"
                :status="store.validationErrors[key] ? 'error' : undefined"
                :input-props="{
                  id: `settings-${key}`,
                  'aria-describedby': store.validationErrors[key]
                    ? `settings-${key}-hint`
                    : 'settings-duration-hint',
                  'aria-invalid': Boolean(store.validationErrors[key]),
                }"
              />
              <span
                v-if="store.validationErrors[key]"
                :id="`settings-${key}-hint`"
                class="phase-d__muted"
                :class="{
                  'settings-page__error': store.validationErrors[key],
                }"
                >{{ t(store.validationErrors[key] || 'durationHint') }}</span
              >
            </div>
          </div>
        </div>
        <div
          v-show="activeTab === 'locking'"
          id="tabpanel-locking"
          class="phase-d__form"
          role="tabpanel"
          aria-labelledby="tab-locking"
        >
          <div class="phase-d__grid">
            <div class="phase-d__field">
              <label
                for="settings-lockOverlap"
                class="phase-d__label"
                >{{ t('lockOverlap') }}</label
              >
              <NInput
                v-model:value="store.draft.lock!.overlap"
                :disabled="isDisabled"
                placeholder="2s"
                :status="
                  store.validationErrors.lockOverlap ? 'error' : undefined
                "
                :input-props="{
                  id: 'settings-lockOverlap',
                  'aria-describedby': 'settings-lockOverlap-hint',
                  'aria-invalid': Boolean(store.validationErrors.lockOverlap),
                }"
              />
              <span
                id="settings-lockOverlap-hint"
                class="phase-d__muted"
                :class="{
                  'settings-page__error': store.validationErrors.lockOverlap,
                }"
                >{{
                  t(store.validationErrors.lockOverlap || 'lockOverlapHint')
                }}</span
              >
            </div>
          </div>
        </div>
        <div
          v-show="activeTab === 'webhook'"
          id="tabpanel-webhook"
          class="phase-d__form"
          role="tabpanel"
          aria-labelledby="tab-webhook"
        >
          <div class="settings-page__switch-row">
            <span
              id="settings-webhook-label"
              class="settings-page__feature-title"
              >{{ t('webhookEnabled') }}</span
            >
            <NSwitch
              v-model:value="store.draft.webhook!.enabled"
              :disabled="isDisabled"
              aria-labelledby="settings-webhook-label"
            />
          </div>
          <div class="phase-d__field settings-page__webhook-field">
            <label
              for="settings-webhookBaseUrl"
              class="phase-d__label"
              >{{ t('webhookBaseUrl') }}</label
            >
            <NInput
              v-model:value="store.draft.webhook!.baseUrl"
              :disabled="isDisabled"
              placeholder="https://tutti.example.com"
              :status="
                store.validationErrors.webhookBaseUrl ? 'error' : undefined
              "
              :input-props="{
                id: 'settings-webhookBaseUrl',
                type: 'url',
                'aria-describedby': 'settings-webhookBaseUrl-hint',
                'aria-invalid': Boolean(store.validationErrors.webhookBaseUrl),
              }"
            />
            <span
              id="settings-webhookBaseUrl-hint"
              class="phase-d__muted"
              :class="{
                'settings-page__error': store.validationErrors.webhookBaseUrl,
              }"
              >{{
                t(store.validationErrors.webhookBaseUrl || 'webhookBaseUrlHint')
              }}</span
            >
          </div>
        </div>
        <div class="settings-page__footer">
          <div class="phase-d__actions">
            <NButton
              type="primary"
              :loading="store.isSaving"
              :disabled="!store.canSave || !auth.isAdmin"
              @click="store.save()"
              >{{ store.isSaving ? t('saving') : t('save') }}</NButton
            >
            <NButton
              :disabled="!store.hasChanges || isDisabled"
              @click="handleReset"
              >{{ t('reset') }}</NButton
            >
            <StatusTag
              v-if="store.saveSuccess && !store.hasChanges"
              variant="success"
              no-dot
              >{{ t('saved') }}</StatusTag
            >
          </div>
          <div
            v-if="store.hasChanges && invalidFields.length"
            class="phase-d__actions"
            role="status"
          >
            <span class="phase-d__muted settings-page__error">{{
              t('validationSummary', { count: invalidFields.length })
            }}</span>
            <NButton
              text
              type="error"
              @click="reviewInvalidFields"
              >{{ t('reviewFields') }}</NButton
            >
          </div>
        </div>
        <p class="phase-d__muted settings-page__apply-note">
          {{ t('effectiveNotice') }}
        </p>
      </SectionCard>
    </form>

    <ConfirmDialog
      :show="showDiscard"
      :title="t('discardTitle')"
      :text="t('discardText')"
      :confirm-label="t('discardConfirm')"
      :cancel-label="t('keepEditing')"
      danger
      @confirm="finishDiscard(true)"
      @update:show="finishDiscard(false)"
    />
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>

<style lang="scss" scoped>
  .settings-page {
    &__form {
      display: flex;
      flex-direction: column;
      gap: 20px;
      min-width: 0;
    }

    &__description {
      margin: 0 0 16px;
      line-height: 20px;
    }

    &__switches {
      display: flex;
      flex-direction: column;
    }

    &__switch-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 24px;
      padding: 12px 0;
      border-bottom: 1px solid var(--tt-border-subtle);

      &:first-child {
        padding-top: 0;
      }

      &:last-child {
        border-bottom: 0;
        padding-bottom: 0;
      }
    }

    &__switch-copy {
      display: flex;
      flex-direction: column;
      min-width: 0;
      gap: 4px;
      line-height: 20px;
    }

    &__feature-title {
      font-size: 13px;
      font-weight: 500;
      color: var(--tt-text);
    }

    &__number {
      width: 100%;
    }

    &__webhook-field {
      margin-top: 16px;
    }

    &__footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
      gap: 12px;
      border-top: 1px solid var(--tt-border);
      margin-top: 24px;
      padding-top: 16px;
    }

    &__apply-note {
      margin: 12px 0 0;
      max-width: 760px;
      line-height: 20px;
    }

    &__error {
      color: var(--tt-danger);
    }
  }
</style>
