<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    NButton,
    NInput,
    NSelect,
    NModal,
    NCheckbox,
    NPopconfirm,
  } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import Callout from '@/components/ui/Callout.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import CopyField from '@/components/ui/CopyField.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { useTuttiConsoleUsersStore } from '@/stores/tuttiConsoleUsersStore';
  import { useAuthStore } from '@/stores/authStore';
  import { TuttiService } from '@/services/TuttiService';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type { ConsoleUser } from '@/utils/types/users';
  const { t } = useI18n({ messages });
  const store = useTuttiConsoleUsersStore();
  const auth = useAuthStore();
  const showForm = ref(false);
  const edited = ref<ConsoleUser | null>(null);
  const username = ref('');
  const role = ref<'admin' | 'viewer'>('viewer');
  const generate = ref(true);
  const password = ref('');
  const oneTime = ref<string | null>(null);
  const stored = ref(false);
  const error = ref('');
  const busy = ref(false);
  const roleOptions = computed(() => [
    { label: t('admin'), value: 'admin' },
    { label: t('viewer'), value: 'viewer' },
  ]);
  const enabledAdmins = computed(
    () => store.users.filter((u) => u.role === 'admin' && !u.disabled).length,
  );

  function protectedAdmin(user: ConsoleUser) {
    return user.role === 'admin' && !user.disabled && enabledAdmins.value <= 1;
  }

  function actions(user: ConsoleUser) {
    return ConsoleUserHelper.actions(user, auth.username);
  }
  onMounted(() => store.load());
  onBeforeUnmount(() => {
    oneTime.value = null;
    password.value = '';
  });

  function open(user: ConsoleUser | null) {
    edited.value = user;
    username.value = user?.username ?? '';
    role.value = user?.role ?? 'viewer';
    generate.value = true;
    password.value = '';
    error.value = '';
    showForm.value = true;
  }

  async function save() {
    error.value = '';

    if (!username.value.trim()) {
      error.value = t('invalidName');

      return;
    }

    if (
      !edited.value &&
      !generate.value &&
      ConsoleUserHelper.validatePassword(password.value, username.value)
    ) {
      error.value = t('passwordInvalid');

      return;
    }

    if (
      edited.value &&
      protectedAdmin(edited.value) &&
      role.value !== 'admin'
    ) {
      error.value = t('lastAdmin');

      return;
    }

    busy.value = true;

    try {
      if (edited.value) {
        await TuttiService.updateConsoleUser({
          username: edited.value.username,
          role: role.value,
        });
      } else {
        const result = await TuttiService.createConsoleUser({
          username: username.value.trim(),
          role: role.value,
          generatePassword: generate.value,
          ...(!generate.value ? { password: password.value } : {}),
        });

        if (result.generatedPassword) {
          oneTime.value = result.generatedPassword;
          stored.value = false;
        }
      }

      showForm.value = false;
      password.value = '';
      await store.load(false);
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }

  async function mutate(
    user: ConsoleUser,
    kind: 'reset' | 'toggle' | 'delete',
  ) {
    error.value = '';
    busy.value = true;

    try {
      if (kind === 'reset') {
        const result = await TuttiService.resetConsoleUserPassword({
          username: user.username,
          generatePassword: true,
        });

        if (result.generatedPassword) {
          oneTime.value = result.generatedPassword;
          stored.value = false;
        }
      } else if (kind === 'toggle') {
        await TuttiService.updateConsoleUser({
          username: user.username,
          disabled: !user.disabled,
        });
      } else {
        await TuttiService.deleteConsoleUser(user.username);
      }

      await store.load(false);
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }
</script>

<template>
  <div class="phase-d">
    <PageHeader
      :title="t('consoleUsers')"
      :description="t('consoleHint')"
      ><template #actions
        ><NButton
          :loading="store.isLoading"
          @click="store.load()"
          >{{ t('refresh') }}</NButton
        ><NButton
          type="primary"
          :disabled="store.authDisabled || store.hasError"
          @click="open(null)"
          >{{ t('invite') }}</NButton
        ></template
      ></PageHeader
    ><Callout
      v-if="store.authDisabled"
      tone="warning"
      >{{ t('authDisabled') }}</Callout
    ><Callout
      v-if="(error && !showForm) || store.hasError"
      tone="danger"
      >{{ error || t('error') }}</Callout
    >
    <SectionCard
      :title="t('consoleUsers')"
      :meta="String(store.users.length)"
      flush
      ><table
        v-if="store.users.length"
        class="phase-d__table"
      >
        <thead>
          <tr>
            <th>{{ t('user') }}</th>
            <th>{{ t('role') }}</th>
            <th>{{ t('status') }}</th>
            <th>{{ t('lastLogin') }}</th>
            <th>{{ t('source') }}</th>
            <th>{{ t('actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="user in store.users"
            :key="user.username"
          >
            <td>
              <strong>{{ user.username }}</strong>
              {{ actions(user).isSelf ? t('self') : '' }}
            </td>
            <td>{{ t(user.role === 'admin' ? 'admin' : 'viewer') }}</td>
            <td>
              <StatusTag
                :variant="
                  user.disabled
                    ? 'neutral'
                    : ConsoleUserHelper.status(user) === 'invited'
                      ? 'info'
                      : 'success'
                "
                >{{ t(ConsoleUserHelper.status(user)) }}</StatusTag
              >
            </td>
            <td>{{ formatDateTime(user.lastLoginAt ?? undefined) }}</td>
            <td>{{ t(user.source === 'config' ? 'config' : 'managed') }}</td>
            <td>
              <span
                v-if="actions(user).isConfig"
                class="phase-d__muted"
                >{{ t('editConfig') }}</span
              >
              <div
                v-else
                class="phase-d__actions"
              >
                <NButton
                  size="small"
                  :disabled="
                    busy || !actions(user).canEdit || protectedAdmin(user)
                  "
                  @click="open(user)"
                  >{{ t('update') }}</NButton
                ><NPopconfirm @positive-click="mutate(user, 'reset')"
                  ><template #trigger
                    ><NButton
                      size="small"
                      :disabled="busy || !actions(user).canResetPassword"
                      >{{ t('resetPassword') }}</NButton
                    ></template
                  >{{ t('resetPassword') }} — {{ user.username }}?</NPopconfirm
                ><NPopconfirm @positive-click="mutate(user, 'toggle')"
                  ><template #trigger
                    ><NButton
                      size="small"
                      :disabled="
                        busy ||
                        !actions(user).canToggleDisabled ||
                        protectedAdmin(user)
                      "
                      >{{ t(user.disabled ? 'enable' : 'disable') }}</NButton
                    ></template
                  >{{ t(user.disabled ? 'enable' : 'disable') }} —
                  {{ user.username }}?</NPopconfirm
                ><NPopconfirm @positive-click="mutate(user, 'delete')"
                  ><template #trigger
                    ><NButton
                      size="small"
                      type="error"
                      secondary
                      :disabled="
                        busy || !actions(user).canDelete || protectedAdmin(user)
                      "
                      >{{ t('delete') }}</NButton
                    ></template
                  >{{
                    t('confirmDelete', { name: user.username })
                  }}</NPopconfirm
                >
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState
        v-else-if="!store.isLoading"
        :text="t('noMatches')"
    /></SectionCard>
    <NModal
      v-model:show="showForm"
      preset="card"
      :title="t(edited ? 'update' : 'invite')"
      style="width: 520px; max-width: 94vw"
      :mask-closable="!busy"
      ><form
        class="phase-d phase-d__form"
        @submit.prevent="save"
      >
        <Callout
          v-if="error"
          tone="danger"
          >{{ error }}</Callout
        ><label class="phase-d__field"
          >{{ t('username')
          }}<NInput
            v-model:value="username"
            :disabled="!!edited || busy"
            autocomplete="off" /></label
        ><label class="phase-d__field"
          >{{ t('role')
          }}<NSelect
            v-model:value="role"
            :options="roleOptions"
            :disabled="busy" /></label
        ><template v-if="!edited"
          ><NCheckbox
            v-model:checked="generate"
            :disabled="busy"
            >{{ t('generate') }}</NCheckbox
          ><label
            v-if="!generate"
            class="phase-d__field"
            >{{ t('password')
            }}<NInput
              v-model:value="password"
              type="password"
              autocomplete="new-password"
              :disabled="busy" /></label></template
        ><NButton
          type="primary"
          attr-type="submit"
          :loading="busy"
          >{{ t('save') }}</NButton
        >
      </form></NModal
    >
    <NModal
      :show="oneTime !== null"
      preset="card"
      :title="t('password')"
      style="width: 520px; max-width: 94vw"
      :mask-closable="false"
      :close-on-esc="false"
      :closable="false"
      ><div class="phase-d">
        <Callout tone="warning">{{ t('oneTime') }}</Callout
        ><CopyField
          v-if="oneTime"
          :value="oneTime"
          :label="t('password')"
          secret
        /><NCheckbox v-model:checked="stored">{{ t('stored') }}</NCheckbox
        ><NButton
          type="primary"
          :disabled="!stored"
          @click="oneTime = null"
          >{{ t('close') }}</NButton
        >
      </div></NModal
    >
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
