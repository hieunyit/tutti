<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import {
    NButton,
    NCheckbox,
    NCollapse,
    NCollapseItem,
    NInput,
    NSelect,
    NSteps,
    NStep,
  } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import BackLink from '@/components/tutti/common/BackLink.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import Callout from '@/components/ui/Callout.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { RuntimeStorageService } from '@/services/RuntimeStorageService';
  import { TuttiService } from '@/services/TuttiService';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { UserHelper, type CredentialRow } from '@/utils/helpers/UserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';
  import { RouteName } from '@/utils/types/router';
  const { t } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const router = useRouter();
  const step = ref(1);
  const name = ref('');
  const rows = ref<CredentialRow[]>([]);
  const busy = ref(false);
  const verifying = ref<string | null>(null);
  const error = ref('');
  const createKey = ref(true);
  const permission = ref('read-write');
  const buckets = ref('*');
  const description = ref('');
  const secret = ref<TuttiProxyAccountSecret | null>(null);
  /** null when the storages could not be loaded: treated as shared */
  const storages = ref<RuntimeStorage[] | null>(null);
  const verticalSteps = ref(false);
  let mobileQuery: MediaQueryList | undefined;
  const permissionOptions = computed(() => [
    { label: t('readWrite'), value: 'read-write' },
    { label: t('readOnly'), value: 'read-only' },
    { label: t('writeOnly'), value: 'write-only' },
  ]);

  onMounted(async () => {
    mobileQuery = window.matchMedia('(max-width: 600px)');
    verticalSteps.value = mobileQuery.matches;
    mobileQuery.addEventListener('change', updateMobileSteps);
    await store.load();
    storages.value = await RuntimeStorageService.list()
      .then((list) => list.storages ?? [])
      .catch(() => null);
    rows.value = UserHelper.newCredentialRows(
      [
        store.mainStorage,
        ...store.storages.filter((s) => s !== store.mainStorage),
      ].filter(Boolean),
    );
  });
  const overridden = computed(() =>
    rows.value.filter(UserHelper.isRowFilled).map((row) => row.storage),
  );
  /** storages the user would share the connection account on */
  const sharedStorages = computed(() =>
    (storages.value ?? [])
      .filter(
        (item) =>
          UserHelper.usesConnectionAccount(item) &&
          !overridden.value.includes(item.name),
      )
      .map((item) => item.name),
  );
  const legacyStorages = computed(() =>
    UserHelper.legacyStorages(storages.value ?? []),
  );
  const policyRequired = computed(() =>
    UserHelper.policyRequired(storages.value, overridden.value),
  );

  onBeforeUnmount(() => {
    mobileQuery?.removeEventListener('change', updateMobileSteps);
    rows.value = [];
    secret.value = null;
  });

  function updateMobileSteps() {
    if (mobileQuery) verticalSteps.value = mobileQuery.matches;
  }

  async function verify(row: CredentialRow) {
    const key = row.accessKeyId;
    const value = row.secretAccessKey;

    verifying.value = row.storage;

    try {
      const result = await TuttiService.testUser({
        name: name.value,
        storage: row.storage,
        accessKeyId: key,
        secretAccessKey: value,
      });

      if (row.accessKeyId === key && row.secretAccessKey === value)
        row.result = { ...result, ok: result.ok === true };
    } catch (e) {
      row.result = {
        ok: false,
        error: ProxyAccountHelper.getMessage(e) ?? t('failed'),
      };
    } finally {
      verifying.value = null;
    }
  }

  function advance() {
    error.value = '';

    if (
      step.value === 1 &&
      UserHelper.validateName(
        name.value,
        store.users.map((u) => u.name),
      )
    ) {
      error.value = t('invalidName');

      return;
    }

    if (step.value === 2) {
      const problem = UserHelper.validatePolicy(
        buckets.value,
        policyRequired.value,
      );

      if (problem) {
        error.value = t(problem);

        return;
      }
    }

    step.value++;
  }

  async function submit() {
    const parsed = UserHelper.parseBucketPatterns(buckets.value);
    const policyProblem = UserHelper.validatePolicy(
      buckets.value,
      policyRequired.value,
    );
    const overrideProblem = UserHelper.validateOverrides(
      rows.value,
      legacyStorages.value,
    );

    if (policyProblem || overrideProblem) {
      error.value = policyProblem
        ? t(policyProblem)
        : overrideProblem === 'incompleteRow'
          ? t('mainRequired')
          : t('legacyRequired', { storages: legacyStorages.value.join(', ') });

      return;
    }

    busy.value = true;
    error.value = '';

    try {
      const result = await TuttiService.createUser({
        name: name.value,
        credentials: UserHelper.toCredentials(rows.value),
        verify: true,
        createAccessKey: createKey.value,
        description: description.value,
        policy: {
          user: name.value,
          permission: permission.value,
          buckets: parsed.patterns.length ? parsed.patterns : ['*'],
        },
      });

      rows.value = [];

      if (result.accessKey) secret.value = result.accessKey;
      else await finish();
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }

  async function finish() {
    secret.value = null;
    await router.push({
      name: RouteName.TUTTI_USER_DETAIL,
      params: { name: name.value },
    });
  }
</script>

<template>
  <div class="phase-d">
    <BackLink
      :to="{ name: RouteName.TUTTI_USERS }"
      :label="t('users')"
    /><PageHeader
      :title="t('createUser')"
      :description="t('createHint')"
    />
    <Callout
      v-if="!store.dynamicEnabled"
      tone="warning"
      >{{ t('notConfigured') }}</Callout
    ><Callout
      v-if="store.hasError && !error"
      tone="danger"
      >{{ t('error') }}</Callout
    >
    <NSteps
      :current="step"
      size="small"
      :vertical="verticalSteps"
      ><NStep :title="t('identity')" /><NStep :title="t('permission')" /><NStep
        :title="t('keys')"
    /></NSteps>
    <SectionCard
      :title="t(step === 1 ? 'identity' : step === 2 ? 'permission' : 'keys')"
      ><form
        class="phase-d__form"
        @submit.prevent="step === 3 ? submit() : advance()"
      >
        <template v-if="step === 1"
          ><label class="phase-d__field"
            >{{ t('name')
            }}<NInput
              v-model:value="name"
              :maxlength="64"
              :input-props="{ 'aria-label': t('name'), autocomplete: 'off' }"
          /></label>
          <p class="phase-d__muted">{{ t('nameHint') }}</p></template
        >
        <template v-if="step === 2"
          ><Callout
            v-if="sharedStorages.length"
            :tone="policyRequired ? 'warning' : 'info'"
            >{{
              t('sharedAccount', { storages: sharedStorages.join(', ') })
            }}</Callout
          ><label class="phase-d__field"
            >{{ t('permission')
            }}<NSelect
              v-model:value="permission"
              :options="permissionOptions" /></label
          ><label class="phase-d__field"
            >{{ t('buckets') }}{{ policyRequired ? ' *' : ''
            }}<NInput
              v-model:value="buckets"
              :input-props="{
                'aria-label': t('buckets'),
                'aria-required': policyRequired,
              }"
          /></label>
          <p class="phase-d__muted">{{ t('bucketsHint') }}</p></template
        >
        <template v-if="step === 3"
          ><NCheckbox v-model:checked="createKey">{{ t('firstKey') }}</NCheckbox
          ><label class="phase-d__field"
            >{{ t('description')
            }}<NInput
              v-model:value="description"
              :input-props="{ 'aria-label': t('description') }"
          /></label>
          <Callout
            v-if="legacyStorages.length"
            tone="warning"
            >{{
              t('legacyRequired', { storages: legacyStorages.join(', ') })
            }}</Callout
          >
          <NCollapse
            :default-expanded-names="legacyStorages.length ? ['advanced'] : []"
          >
            <NCollapseItem
              :title="t('advancedCreds')"
              name="advanced"
            >
              <p class="phase-d__muted">{{ t('advancedHint') }}</p>
              <div
                v-for="row in rows"
                :key="row.storage"
                class="phase-d__panel phase-d__stack"
              >
                <strong
                  >{{ row.storage }}
                  <StatusTag
                    v-if="row.storage === store.mainStorage"
                    variant="info"
                    >{{ t('main') }}</StatusTag
                  ></strong
                >
                <div class="phase-d__grid">
                  <label class="phase-d__field"
                    >{{ t('accessKeyId')
                    }}<NInput
                      v-model:value="row.accessKeyId"
                      :input-props="{
                        'aria-label': `${row.storage} ${t('accessKeyId')}`,
                        autocomplete: 'off',
                      }"
                      @update:value="row.result = null" /></label
                  ><label class="phase-d__field"
                    >{{ t('secret')
                    }}<NInput
                      v-model:value="row.secretAccessKey"
                      type="password"
                      show-password-on="click"
                      :input-props="{
                        'aria-label': `${row.storage} ${t('secret')}`,
                        autocomplete: 'new-password',
                      }"
                      @update:value="row.result = null"
                  /></label>
                </div>
                <div class="phase-d__actions">
                  <NButton
                    :disabled="
                      !UserHelper.isRowFilled(row) || verifying !== null
                    "
                    :loading="verifying === row.storage"
                    @click="verify(row)"
                    >{{ t('verify') }}</NButton
                  ><StatusTag
                    v-if="row.result"
                    :variant="row.result.ok ? 'success' : 'danger'"
                    >{{
                      row.result.ok
                        ? t('verified', { latency: row.result.latencyMs ?? 0 })
                        : row.result.error || t('failed')
                    }}</StatusTag
                  >
                </div>
              </div>
            </NCollapseItem>
          </NCollapse>
        </template>
        <Callout
          v-if="error"
          tone="danger"
          >{{ error }}</Callout
        >
        <div class="phase-d__actions">
          <NButton
            v-if="step > 1"
            :disabled="busy"
            @click="step--"
            >{{ t('back') }}</NButton
          ><NButton
            type="primary"
            attr-type="submit"
            :loading="busy"
            :disabled="
              !store.dynamicEnabled || store.isLoading || store.hasError
            "
            >{{ t(step === 3 ? 'createUser' : 'next') }}</NButton
          >
        </div>
      </form></SectionCard
    ><ProxySecretModal
      :secret="secret"
      kind="created"
      @close="finish"
    />
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
