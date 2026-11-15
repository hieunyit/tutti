<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  /** "Delete user" button plus a confirmation that names the user. */
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { NButton, NCheckbox } from 'naive-ui';
  import { Callout, ConfirmDialog } from '@/components/kit';
  import messages from '@/components/tutti/users/i18nUsers';
  import { TuttiService } from '@/services/TuttiService';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ name: string; disabled: boolean }>();
  const { t } = useI18n({ messages });
  const router = useRouter();
  const open = ref(false);
  const busy = ref(false);
  const error = ref('');
  const force = ref(false);
  const canForce = ref(false);

  function ask() {
    error.value = '';
    force.value = false;
    canForce.value = false;
    open.value = true;
  }

  async function remove() {
    busy.value = true;
    error.value = '';

    try {
      await TuttiService.deleteUser(props.name, force.value);
      await router.push({ name: RouteName.TUTTI_USERS });
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
      canForce.value = UserHelper.parseDeleteBlockers(error.value).canForce;
    } finally {
      busy.value = false;
    }
  }
</script>

<template>
  <div class="user-delete">
    <p>{{ t('deleteUserHint') }}</p>
    <NButton
      type="error"
      secondary
      :disabled="disabled"
      @click="ask"
      >{{ t('deleteUser') }}</NButton
    >
    <ConfirmDialog
      :open="open"
      :title="t('deleteUserTitle', { name })"
      :target-name="name"
      :confirm-label="t('deleteUser')"
      :loading="busy"
      require-typing
      danger
      @cancel="open = false"
      @confirm="remove"
    >
      {{ t('confirmDelete', { name }) }}
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <NCheckbox
        v-if="canForce"
        v-model:checked="force"
        >{{ t('force') }}</NCheckbox
      >
    </ConfirmDialog>
  </div>
</template>

<style scoped lang="scss">
  .user-delete {
    display: flex;
    flex-wrap: wrap;
    gap: var(--tt-space-3);
    align-items: center;
    justify-content: space-between;
    padding-top: var(--tt-space-5);
    border-top: 1px solid var(--tt-border);

    p {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }
  }
</style>
