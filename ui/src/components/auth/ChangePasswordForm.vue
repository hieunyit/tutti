<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput } from 'naive-ui';
  import { useNotification } from '@clyso/clyso-ui-kit';
  import i18nAuth from './i18nAuth';
  import { Callout, FormSection } from '@/components/kit';
  import { useAuthStore } from '@/stores/authStore';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';

  defineProps<{ forced?: boolean }>();
  const emit = defineEmits<{ (e: 'changed'): void }>();
  const { t } = useI18n({ messages: i18nAuth });
  const auth = useAuthStore();
  const { createNotification } = useNotification();
  const current = ref('');
  const password = ref('');
  const repeat = ref('');
  const busy = ref(false);
  const error = ref('');
  const touched = ref({ password: false, repeat: false });
  const strength = computed(() =>
    ConsoleUserHelper.passwordStrength(password.value),
  );
  const passwordError = computed(() =>
    touched.value.password &&
    ConsoleUserHelper.validatePassword(password.value, auth.username ?? '')
      ? t('passwordInvalid')
      : '',
  );
  const repeatError = computed(() =>
    touched.value.repeat && password.value !== repeat.value
      ? t('mismatch')
      : '',
  );
  const rules = computed(() => [
    {
      key: 'length',
      label: t('ruleLength'),
      ok: !ConsoleUserHelper.validatePassword(password.value, ''),
    },
    {
      key: 'different',
      label: t('ruleDifferent'),
      ok: password.value !== '' && password.value !== (auth.username ?? ''),
    },
    {
      key: 'match',
      label: t('ruleMatch'),
      ok: password.value !== '' && password.value === repeat.value,
    },
  ]);

  onBeforeUnmount(() => {
    current.value = '';
    password.value = '';
    repeat.value = '';
  });

  async function submit() {
    error.value = '';
    touched.value = { password: true, repeat: true };

    if (
      ConsoleUserHelper.validatePassword(password.value, auth.username ?? '') ||
      password.value !== repeat.value
    ) {
      return;
    }

    busy.value = true;

    try {
      await auth.changePassword(current.value, password.value);
      current.value = '';
      password.value = '';
      repeat.value = '';
      createNotification({
        type: 'success',
        title: t('passwordChanged'),
        content: t('passwordChanged'),
        isClosable: true,
      });
      emit('changed');
    } catch (e) {
      error.value =
        ProxyAccountHelper.getMessage(e) ?? t('changePasswordFailed');
    } finally {
      busy.value = false;
    }
  }
</script>

<template>
  <form
    class="change-password"
    novalidate
    @submit.prevent="submit"
  >
    <Callout
      v-if="forced"
      tone="warning"
      >{{ t('forcedHint') }}</Callout
    >
    <Callout
      v-if="error"
      tone="danger"
      >{{ error }}</Callout
    >

    <FormSection
      :title="t('changePassword')"
      :description="t('passwordHint')"
    >
      <label class="change-password__field">
        <span>{{ t('currentPassword') }}</span>
        <NInput
          v-model:value="current"
          :input-props="{ autocomplete: 'current-password' }"
          type="password"
          :disabled="busy"
        />
      </label>
      <label class="change-password__field">
        <span>{{ t('newPassword') }}</span>
        <NInput
          v-model:value="password"
          :input-props="{ autocomplete: 'new-password' }"
          type="password"
          show-password-on="click"
          :disabled="busy"
          :status="passwordError ? 'error' : undefined"
          @blur="touched.password = true"
        />
        <span
          v-if="passwordError"
          class="change-password__error"
          role="alert"
          >{{ passwordError }}</span
        >
        <span
          v-else-if="password"
          class="change-password__hint"
          >{{ t('passwordStrength', { strength: t(strength) }) }}</span
        >
      </label>
      <label class="change-password__field">
        <span>{{ t('repeatPassword') }}</span>
        <NInput
          v-model:value="repeat"
          :input-props="{ autocomplete: 'new-password' }"
          type="password"
          :disabled="busy"
          :status="repeatError ? 'error' : undefined"
          @blur="touched.repeat = true"
        />
        <span
          v-if="repeatError"
          class="change-password__error"
          role="alert"
          >{{ repeatError }}</span
        >
      </label>

      <ul
        class="change-password__rules"
        :aria-label="t('passwordRules')"
      >
        <li
          v-for="rule in rules"
          :key="rule.key"
          :class="{ 'change-password__rule--ok': rule.ok }"
        >
          <span aria-hidden="true">{{ rule.ok ? '✓' : '○' }}</span>
          {{ rule.label }}
        </li>
      </ul>
    </FormSection>

    <NButton
      type="primary"
      attr-type="submit"
      :loading="busy"
      :disabled="!current || !password || !repeat"
    >
      {{ busy ? t('changingPassword') : t('changePassword') }}
    </NButton>
  </form>
</template>

<style lang="scss" scoped>
  .change-password {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      margin-bottom: var(--tt-space-3);
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-medium);
    }

    &__error {
      font-size: var(--tt-text-xs);
      font-weight: var(--tt-weight-regular);
      color: var(--tt-danger);
    }

    &__hint {
      font-size: var(--tt-text-xs);
      font-weight: var(--tt-weight-regular);
      color: var(--tt-text-3);
    }

    &__rules {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      margin: 0;
      padding: 0;
      list-style: none;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__rule--ok {
      color: var(--tt-success);
    }
  }
</style>
