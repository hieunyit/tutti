/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import type { DataTableColumns } from 'naive-ui';
import { h } from 'vue';
import { RouterLink } from 'vue-router';
import StorageStatusCell from './StorageStatusCell.vue';
import { storageNotes } from './storageNotes';
import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';
import { RouteName } from '@/utils/types/router';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

export interface StorageColumnDeps {
  t: (key: string, named?: Record<string, unknown>, plural?: number) => string;
  number: (value: number) => string;
  runtime: (name: string) => RuntimeStorage | undefined;
}

export const storageDetailLocation = (storageName: string) => ({
  name: RouteName.TUTTI_STORAGE_DETAILS,
  params: { storageName },
});

/** Table columns of the storages list: name, status, traffic, usage. */
export function storageColumns({
  t,
  number,
  runtime,
}: StorageColumnDeps): DataTableColumns<StorageRow> {
  return [
    {
      title: t('name'),
      key: 'name',
      sorter: 'default',
      render: (row) =>
        h('div', { class: 'storages-cell' }, [
          h(
            RouterLink,
            { to: storageDetailLocation(row.name), class: 'storages-link' },
            () => row.name,
          ),
          h('small', row.address),
        ]),
    },
    {
      title: t('status'),
      key: 'status',
      render: (row) =>
        h(StorageStatusCell, {
          status: row.health.status,
          notes: storageNotes(runtime(row.name), row),
        }),
    },
    {
      title: t('requests'),
      key: 'requestsPerSecond',
      sorter: (a, b) => a.requestsPerSecond - b.requestsPerSecond,
      render: (row) => number(row.requestsPerSecond),
    },
    {
      title: t('usage'),
      key: 'usage',
      render: (row) =>
        h('div', { class: 'storages-cell' }, [
          h(
            RouterLink,
            {
              to: {
                ...storageDetailLocation(row.name),
                query: { tab: 'users' },
              },
              class: 'storages-link',
            },
            () =>
              t('usersCount', { n: number(row.users.count) }, row.users.count),
          ),
          h(
            'small',
            t(
              'replicationsCount',
              {
                n: number(row.replication.sourceOf + row.replication.targetOf),
              },
              row.replication.sourceOf + row.replication.targetOf,
            ),
          ),
        ]),
    },
  ];
}
