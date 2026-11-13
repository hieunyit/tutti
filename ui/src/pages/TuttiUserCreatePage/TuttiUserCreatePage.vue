<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, provide } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import IdentityStep from './IdentityStep.vue';
  import PermissionStep from './PermissionStep.vue';
  import KeysStep from './KeysStep.vue';
  import {
    BackLink,
    Callout,
    PageShell,
    WizardShell,
    type WizardStep,
  } from '@/components/kit';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import messages from '@/components/tutti/users/i18nAccess';
  import {
    USER_CREATE_DRAFT_KEY,
    USER_CREATE_KEY,
    useUserCreate,
    type UserCreateStep,
  } from '@/utils/composables/useUserCreate';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { RouteName } from '@/utils/types/router';

  const { t, te } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const router = useRouter();
  const ctl = useUserCreate();

  provide(USER_CREATE_KEY, ctl);

  const steps = computed<WizardStep[]>(() => [
    { key: 'identity', label: t('identity'), valid: ctl.identityValid.value },
    {
      key: 'permission',
      label: t('permission'),
      valid: ctl.permissionValid.value,
    },
    { key: 'keys', label: t('keys'), valid: true },
  ]);
  const canCreate = computed(
    () => store.dynamicEnabled && !store.hasError && !ctl.busy.value,
  );

  let createdName = '';

  async function finish() {
    if (!canCreate.value) return;

    try {
      const { name } = await ctl.submit();

      createdName = name;

      // no secret to show: go straight to the user
      if (!ctl.created.value?.secretAccessKey) await openUser();
    } catch (e) {
      const message = ProxyAccountHelper.getMessage(e) ?? 'createFailed';

      ctl.error.value = te(message) ? t(message) : message;
    }
  }

  async function openUser() {
    const name = createdName || ctl.form.value.name;

    ctl.dismissSecret();
    await router.push({
      name: RouteName.TUTTI_USER_DETAIL,
      params: { name },
    });
  }

  onMounted(() => ctl.init());
  onBeforeUnmount(() => ctl.dismissSecret());
</script>

<template>
  <PageShell
    :title="t('createUser')"
    :description="t('createHint')"
    state="ready"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.TUTTI_USERS }"
        :label="t('users')"
      />
    </template>
    <Callout
      v-if="!store.dynamicEnabled"
      tone="warning"
      >{{ t('notConfigured') }}</Callout
    >
    <Callout
      v-if="store.hasError && !ctl.error.value"
      tone="danger"
      >{{ t('error') }}</Callout
    >
    <Callout
      v-if="ctl.error.value"
      tone="danger"
      >{{ ctl.error.value }}</Callout
    >
    <WizardShell
      :steps="steps"
      :draft-key="USER_CREATE_DRAFT_KEY"
      :finish-label="t('createUser')"
      :busy="ctl.busy.value || !canCreate"
      @step="ctl.step.value = $event as UserCreateStep"
      @finish="finish"
    >
      <template #identity><IdentityStep /></template>
      <template #permission><PermissionStep /></template>
      <template #keys><KeysStep /></template>
    </WizardShell>
    <ProxySecretModal
      :secret="ctl.created.value?.secretAccessKey ? ctl.created.value : null"
      kind="created"
      @close="openUser"
    />
  </PageShell>
</template>
