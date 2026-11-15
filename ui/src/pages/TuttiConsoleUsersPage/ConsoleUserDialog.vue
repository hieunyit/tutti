<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  /** Create a console user, or change the role of an existing one. */
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput, NModal, NRadio, NRadioGroup } from 'naive-ui';
  import messages from './i18nConsoleUsers';
  import { Callout } from '@/components/kit';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import type { ConsoleUser } from '@/utils/types/users';

  export interface ConsoleUserFormValue {
    username: string;
    role: 'admin' | 'viewer';
    generate: boolean;
    password: string;
  }

  const USERNAME_RULE = /^[A-Za-z0-9][A-Za-z0-9._@-]{0,63}$/;

  const props = defineProps<{
    show: boolean;
    /** the user whose role changes; null creates a user */
    user: ConsoleUser | null;
    busy: boolean;
    /** the last enabled administrator cannot be demoted */
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
  const touched = ref({ username: false, password: false });

  const usernameError = computed(() =>
    !props.user &&
    touched.value.username &&
    !USERNAME_RULE.test(username.value.trim())
      ? t('usernameInvalid')
      : '',
  );
  const passwordError = computed(() =>
    !props.user &&
    !generate.value &&
    touched.value.password &&
    ConsoleUserHelper.validatePassword(password.value, username.value.trim())
      ? t('passwordInvalid')
      : '',
  );
  const roleError = computed(() =>
    props.user && props.protectedAdmin && role.value !== 'admin'
      ? t('lastAdmin')
      : '',
  );

  // a fresh form every time it opens; a typed password never outlives it
  watch(
    () => props.show,
    () => {
      username.value = props.user?.username ?? '';
      role.value = props.user?.role ?? 'viewer';
      generate.value = true;
      password.value = '';
      touched.value = { username: false, password: false };
    },
  );

  function submit() {
    touched.value = { username: true, password: true };

    if (usernameError.value || passwordError.value || roleError.value) return;

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
    :title="t(user ? 'changeRole' : 'create')"
    style="width: 520px; max-width: 94vw"
    :mask-closable="!busy"
    @update:show="!$event && emit('close')"
  >
    <form
      class="console-form"
      novalidate
      @submit.prevent="submit"
    >
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >

      <div
        v-if="!user"
        class="console-form__field"
      >
        <label for="console-username">{{ t('username') }}</label>
        <NInput
          v-model:value="username"
          :input-props="{
            id: 'console-username',
            autocomplete: 'off',
            autocapitalize: 'none',
          }"
          :status="usernameError ? 'error' : undefined"
          :disabled="busy"
          @blur="touched.username = true"
        />
        <span
          v-if="usernameError"
          class="console-form__error"
          role="alert"
          >{{ usernameError }}</span
        >
      </div>

      <fieldset class="console-form__group">
        <legend>{{ t('role') }}</legend>
        <NRadioGroup
          v-model:value="role"
          :disabled="busy"
          name="console-role"
        >
          <NRadio value="viewer">
            <span class="console-form__choice">
              <strong>{{ t('viewer') }}</strong>
              <small>{{ t('viewerHint') }}</small>
            </span>
          </NRadio>
          <NRadio value="admin">
            <span class="console-form__choice">
              <strong>{{ t('admin') }}</strong>
              <small>{{ t('adminHint') }}</small>
            </span>
          </NRadio>
        </NRadioGroup>
        <span
          v-if="roleError"
          class="console-form__error"
          role="alert"
          >{{ roleError }}</span
        >
      </fieldset>

      <fieldset
        v-if="!user"
        class="console-form__group"
      >
        <legend>{{ t('passwordLabel') }}</legend>
        <NRadioGroup
          v-model:value="generate"
          :disabled="busy"
          name="console-password-mode"
        >
          <NRadio :value="true">
            <span class="console-form__choice">
              <strong>{{ t('generate') }}</strong>
              <small>{{ t('generateHint') }}</small>
            </span>
          </NRadio>
          <NRadio :value="false">
            <span class="console-form__choice">
              <strong>{{ t('custom') }}</strong>
            </span>
          </NRadio>
        </NRadioGroup>
        <div
          v-if="!generate"
          class="console-form__field"
        >
          <label for="console-password">{{ t('passwordLabel') }}</label>
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            :input-props="{
              id: 'console-password',
              autocomplete: 'new-password',
            }"
            :status="passwordError ? 'error' : undefined"
            :disabled="busy"
            @blur="touched.password = true"
          />
          <span
            v-if="passwordError"
            class="console-form__error"
            role="alert"
            >{{ passwordError }}</span
          >
          <small
            v-else
            class="console-form__hint"
            >{{ t('passwordHint') }}</small
          >
        </div>
      </fieldset>

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
          >{{ t(user ? 'saveRole' : 'create') }}</NButton
        >
      </div>
    </form>
  </NModal>
</template>

<style lang="scss" scoped>
  .console-form {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-medium);
    }

    &__group {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      min-width: 0;
      margin: 0;
      padding: 0;
      border: 0;

      legend {
        padding: 0;
        margin-bottom: var(--tt-space-2);
        font-size: var(--tt-text-md);
        font-weight: var(--tt-weight-medium);
      }

      :deep(.n-radio-group) {
        display: flex;
        flex-direction: column;
        gap: var(--tt-space-2);
      }
    }

    &__choice {
      display: flex;
      flex-direction: column;

      small {
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
      }
    }

    &__error {
      font-size: var(--tt-text-xs);
      font-weight: var(--tt-weight-regular);
      color: var(--tt-danger);
    }

    &__hint {
      font-weight: var(--tt-weight-regular);
      color: var(--tt-text-3);
    }

    &__actions {
      display: flex;
      gap: var(--tt-space-2);
      justify-content: flex-end;
    }
  }
</style>
