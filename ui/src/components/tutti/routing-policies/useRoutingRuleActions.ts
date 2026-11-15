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

import { useI18n } from 'vue-i18n';
import i18nRouting from '@/pages/TuttiRoutingPage/i18nRouting';
import { useTuttiRoutingPoliciesStore } from '@/stores/tuttiRoutingPoliciesStore';
import {
  useRowActions,
  type RowActionDef,
} from '@/utils/composables/useRowActions';
import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
import type { RoutingPolicy } from '@/utils/types/tuttiCore';

export const routingRuleKey = (row: RoutingPolicy) => row.id;

/**
 * Row and bulk actions of the routing rules list (block, allow, change
 * storage, delete). Every call reports one toast: done, or what failed.
 */
export function useRoutingRuleActions(onEdit: (row: RoutingPolicy) => void) {
  const { t } = useI18n({ messages: i18nRouting });
  const store = useTuttiRoutingPoliciesStore();
  const { createNotification } = useTuttiNotification();

  function report(result: { errorList: [RoutingPolicy, string][] }) {
    const [first] = result.errorList;

    if (first) {
      createNotification({
        type: 'error',
        title: t('actionFailed'),
        content: first[1],
      });
    } else {
      createNotification({
        type: 'success',
        title: t('actionDone'),
        duration: 4000,
      });
    }
  }

  const defs: RowActionDef<RoutingPolicy>[] = [
    {
      key: 'block',
      label: t('block'),
      run: async (row) =>
        report(await store.setRoutingPoliciesBlock([row], true)),
    },
    {
      key: 'unblock',
      label: t('unblock'),
      run: async (row) =>
        report(await store.setRoutingPoliciesBlock([row], false)),
    },
    {
      key: 'edit',
      label: t('changeStorage'),
      run: async (row) => onEdit(row),
    },
    {
      key: 'delete',
      label: t('delete'),
      danger: true,
      run: async (row) => report(await store.deleteRoutingPolicies([row])),
      confirm: () => t('deleteText'),
      disabled: (row) => row.isBlocked,
      disabledReason: () => t('deleteBlocked'),
    },
  ];

  const rowActions = useRowActions<RoutingPolicy>(defs, routingRuleKey);

  /** the block action of an allowed rule, the allow action of a blocked one */
  function actionsFor(row: RoutingPolicy) {
    return rowActions
      .actionsFor(row)
      .filter((action) => action.key !== (row.isBlocked ? 'block' : 'unblock'));
  }

  async function bulkBlock(rows: RoutingPolicy[], block: boolean) {
    report(await store.setRoutingPoliciesBlock(rows, block));
  }

  async function bulkDelete(rows: RoutingPolicy[]) {
    if (rows.some((row) => row.isBlocked)) return;

    report(await store.deleteRoutingPolicies(rows));
  }

  return {
    actionsFor,
    pending: rowActions.pending,
    confirmPending: rowActions.confirmPending,
    cancelPending: rowActions.cancelPending,
    bulkBlock,
    bulkDelete,
  };
}
