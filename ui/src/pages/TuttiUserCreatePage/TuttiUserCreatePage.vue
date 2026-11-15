<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, provide, ref } from 'vue';
  import { RouterLink, useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import AccessSection from './AccessSection.vue';
  import KeySection from './KeySection.vue';
  import NameSection from './NameSection.vue';
  import { BackLink, Callout, PageShell } from '@/components/kit';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import i18nUsers from '@/components/tutti/users/i18nUsers';
  import {
    USER_CREATE_KEY,
    useUserCreate,
  } from '@/utils/composables/useUserCreate';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { RouteName } from '@/utils/types/router';

  const { t, te } = useI18n({ messages: i18nUsers });
  const store = useTuttiUsersStore();
  const router = useRouter();
  const ctl = useUserCreate();
  /** errors show on blur; after a failed submit attempt they show everywhere */
  const attempted = ref(false);

  provide(USER_CREATE_KEY, ctl);

  const canCreate = computed(
    () => store.dynamicEnabled && !store.hasError && !ctl.busy.value,
  );

  let createdName = '';

  async function create() {
    attempted.value = true;

    if (!canCreate.value || !ctl.formValid.value) return;

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
    await router.push({ name: RouteName.TUTTI_USER_DETAIL, params: { name } });
  }

  onMounted(() => ctl.init());
  onBeforeUnmount(() => ctl.dismissSecret());
</script>

<template>
  <PageShell
    :title="t('createUser')"
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
    <form
      class="user-create"
      novalidate
      @submit.prevent="create"
    >
      <NameSection :show-errors="attempted" />
      <AccessSection :show-errors="attempted" />
      <KeySection />
      <div class="user-create__footer">
        <RouterLink :to="{ name: RouteName.TUTTI_USERS }">
          <NButton>{{ t('cancel') }}</NButton>
        </RouterLink>
        <NButton
          type="primary"
          attr-type="submit"
          :loading="ctl.busy.value"
          :disabled="!canCreate"
          >{{ t('createUser') }}</NButton
        >
      </div>
    </form>
    <ProxySecretModal
      :secret="ctl.created.value?.secretAccessKey ? ctl.created.value : null"
      kind="created"
      @close="openUser"
    />
  </PageShell>
</template>

<style lang="scss" scoped>
  .user-create {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    max-width: 720px;

    &__footer {
      display: flex;
      gap: var(--tt-space-2);
      justify-content: flex-end;
    }
  }
</style>
