<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import Callout from '@/components/ui/Callout.vue';
  import TabBar from '@/components/tutti/common/TabBar.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { RouteName } from '@/utils/types/router';
  const { t } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const search = ref('');
  const tab = ref('all');
  const visible = computed(() =>
    store.users.filter(
      (user) =>
        user.name.toLowerCase().includes(search.value.toLowerCase()) &&
        (tab.value !== 'incomplete' || UserHelper.isIncomplete(user)),
    ),
  );

  onMounted(() => store.load());
</script>

<template>
  <div class="phase-d">
    <PageHeader
      :title="t('users')"
      :description="t('usersHint')"
      ><template #actions
        ><NButton
          :loading="store.isLoading"
          @click="store.load()"
          >{{ t('refresh') }}</NButton
        ><WriteGate
          ><RouterLink :to="{ name: RouteName.TUTTI_USER_CREATE }"
            ><NButton type="primary">{{ t('createUser') }}</NButton></RouterLink
          ></WriteGate
        ></template
      ></PageHeader
    >
    <Callout
      v-if="!store.dynamicEnabled"
      tone="warning"
      >{{ t('notConfigured') }}</Callout
    >
    <Callout
      v-for="user in store.incompleteUsers"
      :key="user.name"
      tone="warning"
      ><RouterLink
        :to="{ name: RouteName.TUTTI_USER_DETAIL, params: { name: user.name } }"
        >{{ user.name }}</RouterLink
      >
      —
      {{
        t('missing', { storages: UserHelper.missingStorages(user).join(', ') })
      }}</Callout
    >
    <Callout
      v-if="store.hasError"
      tone="danger"
      >{{ t('error') }}
      <NButton
        text
        @click="store.load()"
        >{{ t('retry') }}</NButton
      ></Callout
    >
    <SectionCard
      :title="t('users')"
      flush
    >
      <div
        class="phase-d__toolbar"
        style="padding: 16px"
      >
        <NInput
          v-model:value="search"
          class="phase-d__input"
          :placeholder="t('search')"
          :aria-label="t('search')"
        /><TabBar
          v-model="tab"
          :label="t('users')"
          :tabs="[
            { value: 'all', label: t('all'), count: store.users.length },
            {
              value: 'incomplete',
              label: t('incomplete'),
              count: store.incompleteUsers.length,
            },
          ]"
        />
      </div>
      <table
        v-if="visible.length"
        class="phase-d__table"
      >
        <thead>
          <tr>
            <th>{{ t('user') }}</th>
            <th>{{ t('credentials') }}</th>
            <th>{{ t('permission') }}</th>
            <th>{{ t('keys') }}</th>
            <th>{{ t('source') }}</th>
            <th>{{ t('actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="user in visible"
            :key="user.name"
          >
            <td>
              <RouterLink
                :to="{
                  name: RouteName.TUTTI_USER_DETAIL,
                  params: { name: user.name },
                }"
                ><strong>{{ user.name }}</strong></RouterLink
              >
            </td>
            <td>
              <div class="phase-d__actions">
                <StatusTag
                  v-for="part in UserHelper.credentialParts(user)"
                  :key="part.kind"
                  :variant="
                    part.kind === 'connection-account'
                      ? 'success'
                      : part.kind === 'override'
                        ? 'info'
                        : 'warning'
                  "
                  >{{ UserHelper.credentialText(part, t) }}</StatusTag
                >
              </div>
            </td>
            <td>
              {{ user.policy?.permission ?? 'read-write' }}
              <div class="phase-d__muted">
                {{ user.policy?.buckets?.join(', ') || '*' }}
              </div>
            </td>
            <td>
              {{
                (user.accessKeys?.static ?? 0) +
                (user.accessKeys?.dynamicActive ?? 0) +
                (user.accessKeys?.dynamicDisabled ?? 0)
              }}
            </td>
            <td>
              {{
                t(
                  user.storages?.some((s) => s.source === 'config')
                    ? 'config'
                    : 'managed',
                )
              }}
            </td>
            <td>
              <RouterLink
                :to="{
                  name: RouteName.TUTTI_USER_DETAIL,
                  params: { name: user.name },
                }"
                >{{ t('credentials') }}</RouterLink
              >
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState
        v-else-if="!store.isLoading"
        :text="t(store.users.length ? 'noMatches' : 'emptyUsers')"
      />
    </SectionCard>
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
