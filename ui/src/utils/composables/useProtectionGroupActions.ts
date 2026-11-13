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

import type { Ref } from 'vue';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';

export interface ProtectionGroupActions {
  switchover: (target: string) => Promise<void>;
  abortSwitch: () => Promise<void>;
  clearWriter: (token: string) => Promise<void>;
  remove: () => Promise<void>;
}

/**
 * Dangerous actions of one protection group. Each wraps the service call and
 * then refreshes the protection store; a failed call rejects (the caller
 * shows the message) and nothing is reloaded.
 */
export function useProtectionGroupActions(
  name: Ref<string>,
): ProtectionGroupActions {
  const store = useTuttiProtectionStore();

  async function switchover(target: string) {
    await ProtectionGroupService.switchover(name.value, target);
    await store.loadDetail(name.value, true);
  }

  async function abortSwitch() {
    await ProtectionGroupService.abortSwitch(name.value);
    await store.loadDetail(name.value, true);
  }

  async function clearWriter(token: string) {
    await ProtectionGroupService.clearWriter({
      id: name.value,
      token,
      worker: token in (store.writers.workers ?? {}),
      externallyFenced: true,
    });
    await store.loadWriters(name.value);
  }

  async function remove() {
    await ProtectionGroupService.remove(name.value);
    await store.refresh();
  }

  return { switchover, abortSwitch, clearWriter, remove };
}
