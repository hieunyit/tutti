<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NCheckbox, NInput, NModal, NSelect } from 'naive-ui';
  import messages from './i18nAccess';
  import { Callout } from '@/components/kit';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import type { ConsoleUser } from '@/utils/types/users';

  export interface ConsoleUserFormValue {
    username: string;
    role: 'admin' | 'viewer';
    generate: boolean;
    password: string;
  }

  const props = defineProps<{
    show: boolean;
    /** the user whose role is changed; null creates a user */
    user: ConsoleUser | null;
    busy: boolean;
    /** the last admin cannot be demoted */
    protectedAdmin: boolean;
    error: string;
  }>();

  const emit = defineEmits<{
    (e: 'save', value: ConsoleUserFormValue): void;
    (e: 'close'): void;
  }>();

  const { t } = useI18n({ messages });
  const username = ref('');
  const role = ref<'admin' | 'viewer'>('viewer');
  const generate = ref(true);
  const password = ref('');
  const touched = ref(false);

  const roleOptions = computed(() => [
    { label: t('admin'), value: 'admin' },
    { label: t('viewer'), value: 'viewer' },
  ]);
  const problem = computed(() => {
    if (!username.value.trim()) return t('invalidName');

    if (
      !props.user &&
      !generate.value &&
      ConsoleUserHelper.validatePassword(password.value, username.value)
    )
      return t('passwordInvalid');

    if (props.user && props.protectedAdmin && role.value !== 'admin')
      return t('lastAdmin');

    return '';
  });

  // a fresh form every time it opens; the typed password is dropped on close
  watch(
    () => props.show,
    (open) => {
      username.value = props.user?.username ?? '';
      role.value = props.user?.role ?? 'viewer';
      generate.value = true;
      password.value = '';
      touched.value = false;

      if (!open) password.value = '';
    },
  );

  function submit() {
    touched.value = true;

    if (problem.value) return;

    emit('save', {
      username: username.value.trim(),
      role: role.value,
      generate: generate.value,
      password: password.value,
    });
  }
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t(user ? 'changeRole' : 'createConsoleUser')"
    style="width: 520px; max-width: 94vw"
    :mask-closable="!busy"
    @update:show="!$event && emit('close')"
  >
    <form
      class="console-form"
      @submit.prevent="submit"
    >
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <label class="console-form__field">
        {{ t('username') }}
        <NInput
          v-model:value="username"
          :disabled="!!user || busy"
          :input-props="{ autocomplete: 'off' }"
        />
      </label>
      <label class="console-form__field">
        {{ t('role') }}
        <NSelect
          v-model:value="role"
          :options="roleOptions"
          :disabled="busy"
        />
      </label>
      <template v-if="!user">
        <NCheckbox
          v-model:checked="generate"
          :disabled="busy"
          >{{ t('generate') }}</NCheckbox
        >
        <label
          v-if="!generate"
          class="console-form__field"
        >
          {{ t('password') }}
          <NInput
            v-model:value="password"
            type="password"
            :input-props="{ autocomplete: 'new-password' }"
            :disabled="busy"
          />
        </label>
      </template>
      <p
        v-if="touched && problem"
        class="console-form__error"
        role="alert"
      >
        {{ problem }}
      </p>
      <div class="console-form__actions">
        <NButton
          :disabled="busy"
          @click="emit('close')"
          >{{ t('cancel') }}</NButton
        >
        <NButton
          type="primary"
          attr-type="submit"
          :loading="busy"
          >{{ t(user ? 'saveRole' : 'createConsoleUser') }}</NButton
        >
      </div>
    </form>
  </NModal>
</template>

<style lang="scss" scoped>
  .console-form {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      font-size: var(--tt-text-sm);
    }

    &__error {
      margin: 0;
      font-size: var(--tt-text-xs);
      color: var(--tt-danger);
    }

    &__actions {
      display: flex;
      gap: var(--tt-space-2);
      justify-content: flex-end;
    }
  }
</style>
