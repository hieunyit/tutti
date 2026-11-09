<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton, NCheckbox, NInput, NSelect, NSteps, NStep } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import BackLink from '@/components/tutti/common/BackLink.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import Callout from '@/components/ui/Callout.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { TuttiService } from '@/services/TuttiService';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { UserHelper, type CredentialRow } from '@/utils/helpers/UserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
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
  const permissionOptions = computed(() => [
    { label: t('readWrite'), value: 'read-write' },
    { label: t('readOnly'), value: 'read-only' },
    { label: t('writeOnly'), value: 'write-only' },
  ]);

  onMounted(async () => {
    await store.load();
    rows.value = UserHelper.newCredentialRows(
      [
        store.mainStorage,
        ...store.storages.filter((s) => s !== store.mainStorage),
      ].filter(Boolean),
    );
  });
  onBeforeUnmount(() => {
    rows.value = [];
    secret.value = null;
  });

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
      if (UserHelper.validateCredentials(rows.value, store.mainStorage)) {
        error.value = t('mainRequired');

        return;
      }

      if (
        rows.value.filter(UserHelper.isRowFilled).some((r) => !r.result?.ok)
      ) {
        error.value = t('verifyBefore');

        return;
      }
    }

    step.value++;
  }

  async function submit() {
    const parsed = UserHelper.parseBucketPatterns(buckets.value);

    if (parsed.invalid.length) {
      error.value = t('invalidBuckets');

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
      v-if="store.hasError || error"
      tone="danger"
      >{{ error || t('error') }}</Callout
    >
    <NSteps
      :current="step"
      size="small"
      ><NStep :title="t('identity')" /><NStep :title="t('credentials')" /><NStep
        :title="t('keys')"
    /></NSteps>
    <SectionCard
      :title="
        t(step === 1 ? 'identity' : step === 2 ? 'credentials' : 'permission')
      "
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
              autocomplete="off"
          /></label>
          <p class="phase-d__muted">{{ t('nameHint') }}</p></template
        >
        <template v-if="step === 2"
          ><Callout>{{ t('credentialHint') }}</Callout>
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
                  autocomplete="off"
                  @update:value="row.result = null" /></label
              ><label class="phase-d__field"
                >{{ t('secret')
                }}<NInput
                  v-model:value="row.secretAccessKey"
                  type="password"
                  show-password-on="click"
                  autocomplete="new-password"
                  @update:value="row.result = null"
              /></label>
            </div>
            <div class="phase-d__actions">
              <NButton
                :disabled="!UserHelper.isRowFilled(row) || verifying !== null"
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
          <NButton @click="rows = UserHelper.applySameCredentials(rows)">{{
            t('same')
          }}</NButton
          ><Callout tone="warning">{{ t('skipHint') }}</Callout></template
        >
        <template v-if="step === 3"
          ><NCheckbox v-model:checked="createKey">{{ t('firstKey') }}</NCheckbox
          ><label class="phase-d__field"
            >{{ t('description') }}<NInput v-model:value="description" /></label
          ><label class="phase-d__field"
            >{{ t('permission')
            }}<NSelect
              v-model:value="permission"
              :options="permissionOptions" /></label
          ><label class="phase-d__field"
            >{{ t('buckets') }}<NInput v-model:value="buckets"
          /></label>
          <p class="phase-d__muted">{{ t('bucketsHint') }}</p>
          <Callout
            v-if="UserHelper.skippedStorages(rows).length"
            tone="warning"
            >{{
              t('missing', {
                storages: UserHelper.skippedStorages(rows).join(', '),
              })
            }}</Callout
          ></template
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
