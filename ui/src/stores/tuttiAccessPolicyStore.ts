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

import { defineStore } from 'pinia';
import { reactive, toRefs } from 'vue';
import { TuttiService } from '@/services/TuttiService';
import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
import type {
  TuttiAccessPolicyForm,
  TuttiProxyAccount,
} from '@/utils/types/tutti';

interface TuttiAccessPolicyState {
  isLoading: boolean;
  hasError: boolean;
  accounts: TuttiProxyAccount[];
  isSaving: boolean;
  /** backend validation message, or 'unknown' */
  saveError: string | null;
}

function getInitialState(): TuttiAccessPolicyState {
  return {
    isLoading: false,
    hasError: false,
    accounts: [],
    isSaving: false,
    saveError: null,
  };
}

export const useTuttiAccessPolicyStore = defineStore(
  'tuttiAccessPolicy',
  () => {
    const state = reactive<TuttiAccessPolicyState>(getInitialState());

    async function reloadAccounts() {
      const res = await TuttiService.getProxyAccounts();

      state.accounts = res.accounts ?? [];
    }

    async function init() {
      state.isLoading = true;
      state.hasError = false;

      try {
        await reloadAccounts();
      } catch {
        state.hasError = true;
      } finally {
        state.isLoading = false;
      }
    }

    /** Returns true on success; on failure sets saveError. */
    async function savePolicy(form: TuttiAccessPolicyForm): Promise<boolean> {
      state.isSaving = true;
      state.saveError = null;

      try {
        await TuttiService.setAccessPolicy(
          AccessPolicyHelper.formToPolicy(form),
        );
        await reloadAccounts();

        return true;
      } catch (e) {
        state.saveError =
          AccessPolicyHelper.getValidationMessage(e) ?? 'unknown';

        return false;
      } finally {
        state.isSaving = false;
      }
    }

    async function removePolicy(user: string): Promise<boolean> {
      try {
        await TuttiService.deleteAccessPolicy(user);
        await reloadAccounts();

        return true;
      } catch {
        return false;
      }
    }

    function $reset() {
      Object.assign(state, getInitialState());
    }

    return { ...toRefs(state), init, savePolicy, removePolicy, $reset };
  },
);
