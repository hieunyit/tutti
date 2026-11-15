<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Users as cards on narrow screens (the table needs horizontal scroll there). */
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import i18nUsers from './i18nUsers';
  import {
    bucketsLabel,
    detailLink,
    incompleteBadge,
    keyCount,
    keysLink,
    originLabel,
    permissionLabel,
  } from './userColumns';
  import type { S3User } from '@/utils/types/users';

  defineProps<{ users: S3User[] }>();
  const { t } = useI18n({ messages: i18nUsers });
  const Badge = (props: { user: S3User }) => incompleteBadge(props.user, t);
</script>

<template>
  <ul class="user-cards">
    <li
      v-for="user in users"
      :key="user.name"
      class="user-cards__item"
    >
      <RouterLink
        class="user-cards__name"
        :to="detailLink(user.name)"
        >{{ user.name }}</RouterLink
      >
      <Badge :user="user" />
      <small
        >{{ permissionLabel(user, t) }} · {{ bucketsLabel(user, t) }}</small
      >
      <small
        >{{ originLabel(user, t) }} ·
        <RouterLink :to="keysLink(user.name)">{{
          t('keysCount', { count: keyCount(user) }, keyCount(user))
        }}</RouterLink></small
      >
    </li>
  </ul>
</template>

<style scoped lang="scss">
  .user-cards {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;
    padding: 0;
    list-style: none;

    &__item {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--tt-space-1);
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);

      small {
        max-width: 100%;
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
        overflow-wrap: anywhere;
      }
    }

    &__name {
      color: var(--tt-accent);
      font-weight: var(--tt-weight-strong);
      text-decoration: none;
    }
  }
</style>
