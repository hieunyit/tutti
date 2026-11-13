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

import { computed, ref, type Ref } from 'vue';

export interface RowActionDef<T> {
  key: string;
  label: string;
  danger?: boolean;
  run: (row: T) => Promise<void>;
  /** consequence text; with `danger` the action asks for confirmation first */
  confirm?: (row: T) => string;
}

export interface RowAction {
  key: string;
  label: string;
  danger: boolean;
  /** this action is running for this row */
  busy: boolean;
  invoke: () => void;
}

export interface PendingRowAction<T> {
  def: RowActionDef<T>;
  row: T;
  message: string;
}

/**
 * Row actions of a table: one definition list shared by every row. A
 * `danger` action with `confirm` is parked in `pending` (bind it to a
 * ConfirmDialog) and runs only after `confirmPending()`; any other action
 * runs at once. `running` holds "action:rowKey" while a call is in flight.
 */
export function useRowActions<T>(
  defs: RowActionDef<T>[],
  rowKey: (row: T) => string = (row) => String(row),
) {
  const running = ref(new Set<string>()) as Ref<Set<string>>;
  const pending = ref<PendingRowAction<T> | null>(
    null,
  ) as Ref<PendingRowAction<T> | null>;
  const isConfirming = computed(() => pending.value !== null);

  async function execute(def: RowActionDef<T>, row: T) {
    const id = `${def.key}:${rowKey(row)}`;

    if (running.value.has(id)) return;

    running.value = new Set(running.value).add(id);

    try {
      await def.run(row);
    } finally {
      const next = new Set(running.value);

      next.delete(id);
      running.value = next;
    }
  }

  function invoke(def: RowActionDef<T>, row: T) {
    if (def.danger && def.confirm) {
      pending.value = { def, row, message: def.confirm(row) };

      return;
    }

    // run() reports its own failures (notification); nothing to rethrow here
    execute(def, row).catch(() => undefined);
  }

  async function confirmPending() {
    const current = pending.value;

    pending.value = null;

    if (current) await execute(current.def, current.row).catch(() => undefined);
  }

  function cancelPending() {
    pending.value = null;
  }

  function actionsFor(row: T): RowAction[] {
    return defs.map((def) => ({
      key: def.key,
      label: def.label,
      danger: def.danger ?? false,
      busy: running.value.has(`${def.key}:${rowKey(row)}`),
      invoke: () => invoke(def, row),
    }));
  }

  return {
    actionsFor,
    running,
    pending,
    isConfirming,
    confirmPending,
    cancelPending,
  };
}
