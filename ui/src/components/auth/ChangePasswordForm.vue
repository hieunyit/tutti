<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput } from 'naive-ui';
  import Callout from '@/components/ui/Callout.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { useAuthStore } from '@/stores/authStore';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  defineProps<{ forced?: boolean }>();
  const emit = defineEmits<{ (e: 'changed'): void }>();
  const { t } = useI18n({ messages });
  const auth = useAuthStore();
  const current = ref('');
  const password = ref('');
  const repeat = ref('');
  const busy = ref(false);
  const error = ref('');
  const strength = computed(() =>
    ConsoleUserHelper.passwordStrength(password.value),
  );

  onBeforeUnmount(() => {
    current.value = '';
    password.value = '';
    repeat.value = '';
  });

  async function submit() {
    error.value = '';

    if (
      ConsoleUserHelper.validatePassword(password.value, auth.username ?? '')
    ) {
      error.value = t('passwordInvalid');

      return;
    }

    if (password.value !== repeat.value) {
      error.value = t('mismatch');

      return;
    }

    busy.value = true;

    try {
      await auth.changePassword(current.value, password.value);
      current.value = '';
      password.value = '';
      repeat.value = '';
      emit('changed');
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }
</script>

<template>
  <form
    class="phase-d phase-d__form"
    @submit.prevent="submit"
  >
    <Callout
      v-if="forced"
      tone="warning"
      >{{ t('forcedHint') }}</Callout
    >
    <p class="phase-d__muted">{{ t('passwordHint') }}</p>
    <Callout
      v-if="error"
      tone="danger"
      >{{ error }}</Callout
    ><label class="phase-d__field"
      >{{ t('currentPassword')
      }}<NInput
        v-model:value="current"
        :input-props="{
          'aria-label': t('currentPassword'),
          autocomplete: 'current-password',
        }"
        type="password"
        :disabled="busy" /></label
    ><label class="phase-d__field"
      >{{ t('newPassword')
      }}<NInput
        v-model:value="password"
        :input-props="{
          'aria-label': t('newPassword'),
          autocomplete: 'new-password',
        }"
        type="password"
        show-password-on="click"
        :disabled="busy"
    /></label>
    <p
      v-if="password"
      class="phase-d__muted"
    >
      {{ t('passwordStrength', { strength: t(strength) }) }}
    </p>
    <label class="phase-d__field"
      >{{ t('repeatPassword')
      }}<NInput
        v-model:value="repeat"
        :input-props="{
          'aria-label': t('repeatPassword'),
          autocomplete: 'new-password',
        }"
        type="password"
        :disabled="busy" /></label
    ><NButton
      type="primary"
      attr-type="submit"
      :loading="busy"
      :disabled="!current || !password || !repeat"
      >{{ t('changePassword') }}</NButton
    >
  </form>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
