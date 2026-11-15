/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { h } from 'vue';
import { RouterLink } from 'vue-router';
import type { DataTableColumns } from 'naive-ui';
import { StatusBadge } from '@/components/kit';
import { UserHelper } from '@/utils/helpers/UserHelper';
import { RouteName } from '@/utils/types/router';
import type { S3User } from '@/utils/types/users';

type Translate = (key: string, params?: Record<string, unknown>) => string;

export function keyCount(user: S3User): number {
  return (
    (user.accessKeys?.static ?? 0) +
    (user.accessKeys?.dynamicActive ?? 0) +
    (user.accessKeys?.dynamicDisabled ?? 0)
  );
}

export function permissionLabel(user: S3User, t: Translate): string {
  const key =
    {
      'read-only': 'readOnly',
      'write-only': 'writeOnly',
    }[user.policy?.permission ?? ''] ?? 'readWrite';

  return t(key);
}

/** "all buckets" or the patterns; never the raw `*`. */
export function bucketsLabel(user: S3User, t: Translate): string {
  const buckets = user.policy?.buckets ?? [];

  return buckets.length === 0 || buckets.includes('*')
    ? t('allBuckets')
    : buckets.join(', ');
}

export function originLabel(user: S3User, t: Translate): string {
  return t(
    user.storages?.some((s) => s.source === 'config') ? 'config' : 'managed',
  );
}

export const detailLink = (name: string) => ({
  name: RouteName.TUTTI_USER_DETAIL,
  params: { name },
});

export const keysLink = (user: string) => ({
  name: RouteName.TUTTI_ACCESS_KEYS,
  query: { user },
});

/** Warning badge, only when the user lacks credentials on some storage. */
export function incompleteBadge(user: S3User, t: Translate) {
  const missing = UserHelper.missingStorages(user);

  return UserHelper.isIncomplete(user)
    ? h(StatusBadge, {
        tone: 'warning',
        label: missing.length
          ? t('missingOn', { storages: missing.join(', ') })
          : t('statusIncomplete'),
      })
    : null;
}

export function userColumns(t: Translate): DataTableColumns<S3User> {
  return [
    {
      title: t('user'),
      key: 'name',
      sorter: 'default',
      render: (user) =>
        h('div', { class: 'users-cell' }, [
          h(RouterLink, { to: detailLink(user.name) }, () =>
            h('strong', user.name),
          ),
          incompleteBadge(user, t),
        ]),
    },
    {
      title: t('permission'),
      key: 'permission',
      render: (user) =>
        h('div', { class: 'users-cell' }, [
          permissionLabel(user, t),
          h('small', bucketsLabel(user, t)),
        ]),
    },
    {
      title: t('keys'),
      key: 'keys',
      align: 'right',
      render: (user) =>
        h(RouterLink, { to: keysLink(user.name) }, () =>
          String(keyCount(user)),
        ),
    },
    {
      title: t('origin'),
      key: 'origin',
      render: (user) => originLabel(user, t),
    },
  ];
}
