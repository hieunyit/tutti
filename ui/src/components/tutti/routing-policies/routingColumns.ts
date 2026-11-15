/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { h } from 'vue';
import type { DataTableColumns } from 'naive-ui';
import RoutingRuleMenu from './RoutingRuleMenu.vue';
import { routingRuleKey } from './useRoutingRuleActions';
import { StatusBadge } from '@/components/kit';
import type { RowAction } from '@/utils/composables/useRowActions';
import type { RoutingPolicy } from '@/utils/types/tuttiCore';

export const ALL_BUCKETS = '*';

interface ColumnOptions {
  t: (key: string) => string;
  actionsFor: (row: RoutingPolicy) => RowAction[];
}

/** Columns of the routing rules table: user, bucket, storage, status, menu. */
export function routingColumns({
  t,
  actionsFor,
}: ColumnOptions): DataTableColumns<RoutingPolicy> {
  return [
    { title: t('colUser'), key: 'user', minWidth: 140 },
    {
      title: t('colBucket'),
      key: 'bucket',
      minWidth: 160,
      render: (row) =>
        row.bucket === ALL_BUCKETS
          ? h('span', { title: t('allBucketsHint') }, t('allBuckets'))
          : row.bucket,
    },
    {
      title: t('colStorage'),
      key: 'toStorage',
      minWidth: 140,
      render: (row) => row.toStorage || t('noStorage'),
    },
    {
      title: t('colStatus'),
      key: 'isBlocked',
      width: 130,
      render: (row) =>
        h(StatusBadge, {
          tone: row.isBlocked ? 'danger' : 'success',
          label: row.isBlocked ? t('statusBlocked') : t('statusAllowed'),
        }),
    },
    {
      title: '',
      key: 'actions',
      width: 64,
      render: (row) =>
        h(RoutingRuleMenu, {
          actions: actionsFor(row),
          label: row.user,
          key: routingRuleKey(row),
        }),
    },
  ];
}
